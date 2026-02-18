// Package services implémente la logique métier.
package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"moxa-config/internal/models"
)

const interfacesFile = "/etc/network/interfaces"

// InterfaceBackup représente la sauvegarde de config d'une interface.
type InterfaceBackup struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
	CIDR    int    `json:"cidr"`
}

// BridgeBackup représente la sauvegarde complète d'un bridge.
type BridgeBackup struct {
	BridgeName         string            `json:"bridgeName"`
	Interfaces         []InterfaceBackup `json:"interfaces"`
	OriginalInterfaces string            `json:"originalInterfacesFile"`
}

// BridgeService gère les opérations sur les bridges réseau.
type BridgeService struct {
	configPath string
	backupPath string
}

// NewBridgeService crée une instance du service bridge.
func NewBridgeService(configPath, backupPath string) *BridgeService {
	return &BridgeService{
		configPath: configPath,
		backupPath: backupPath,
	}
}

// ListBridges retourne la liste des bridges réseau existants.
func (s *BridgeService) ListBridges() ([]models.Bridge, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("impossible de lister les interfaces: %w", err)
	}

	var bridges []models.Bridge
	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "br") {
			continue
		}

		bridge, err := s.GetBridge(iface.Name)
		if err != nil {
			continue
		}
		bridges = append(bridges, *bridge)
	}

	return bridges, nil
}

// GetBridge retourne la configuration d'un bridge spécifique.
func (s *BridgeService) GetBridge(name string) (*models.Bridge, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("bridge non trouvé: %w", err)
	}

	bridge := &models.Bridge{
		Name:       iface.Name,
		MAC:        iface.HardwareAddr.String(),
		Status:     s.getStatus(iface.Flags),
		Interfaces: s.getBridgeSlaves(name),
		DNS:        s.readDNS(),
	}

	addrs, _ := iface.Addrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			bridge.IP = ipnet.IP.String()
			bridge.Netmask = net.IP(ipnet.Mask).String()
			break
		}
	}

	bridge.Gateway = s.getGateway(name)

	return bridge, nil
}

// CreateBridge crée un nouveau bridge avec les interfaces spécifiées.
func (s *BridgeService) CreateBridge(cfg models.Bridge) error {
	if err := s.validateBridgeConfig(cfg); err != nil {
		return err
	}

	if _, err := net.InterfaceByName(cfg.Name); err == nil {
		return fmt.Errorf("le bridge %s existe déjà", cfg.Name)
	}

	for _, ifaceName := range cfg.Interfaces {
		if master := s.GetBridgeMaster(ifaceName); master != "" {
			return fmt.Errorf("l'interface %s est déjà dans le bridge %s", ifaceName, master)
		}
	}

	// Sauvegarder la config réseau live de chaque interface + fichier interfaces
	backup := BridgeBackup{
		BridgeName: cfg.Name,
		Interfaces: make([]InterfaceBackup, 0, len(cfg.Interfaces)),
	}

	for _, ifaceName := range cfg.Interfaces {
		ifaceBackup := s.captureInterfaceConfig(ifaceName)
		backup.Interfaces = append(backup.Interfaces, ifaceBackup)
	}

	// Sauvegarder le fichier interfaces original
	originalContent, err := os.ReadFile(interfacesFile)
	if err == nil {
		backup.OriginalInterfaces = string(originalContent)
	}

	if err := s.saveBackup(backup); err != nil {
		return fmt.Errorf("erreur sauvegarde config: %w", err)
	}

	// Créer le bridge avec les commandes ip
	if err := exec.Command("ip", "link", "add", "name", cfg.Name, "type", "bridge").Run(); err != nil {
		return fmt.Errorf("erreur création bridge: %w", err)
	}

	for _, ifaceName := range cfg.Interfaces {
		exec.Command("ip", "addr", "flush", "dev", ifaceName).Run()

		if err := exec.Command("ip", "link", "set", ifaceName, "master", cfg.Name).Run(); err != nil {
			exec.Command("ip", "link", "del", cfg.Name).Run()
			return fmt.Errorf("erreur ajout interface %s au bridge: %w", ifaceName, err)
		}

		exec.Command("ip", "link", "set", ifaceName, "up").Run()
	}

	cidr := maskToCIDR(cfg.Netmask)
	if err := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", cfg.IP, cidr), "dev", cfg.Name).Run(); err != nil {
		exec.Command("ip", "link", "del", cfg.Name).Run()
		return fmt.Errorf("erreur configuration IP bridge: %w", err)
	}

	if err := exec.Command("ip", "link", "set", cfg.Name, "up").Run(); err != nil {
		exec.Command("ip", "link", "del", cfg.Name).Run()
		return fmt.Errorf("erreur activation bridge: %w", err)
	}

	if cfg.Gateway != "" {
		exec.Command("ip", "route", "del", "default").Run()
		if err := exec.Command("ip", "route", "add", "default", "via", cfg.Gateway, "dev", cfg.Name).Run(); err != nil {
			return fmt.Errorf("erreur configuration gateway: %w", err)
		}
	}

	if len(cfg.DNS) > 0 {
		s.writeDNS(cfg.DNS)
	}

	// Persister dans /etc/network/interfaces
	return s.persistBridgeToInterfaces(cfg)
}

// DeleteBridge supprime un bridge et restaure la config de toutes les interfaces.
func (s *BridgeService) DeleteBridge(name string) error {
	bridge, err := s.GetBridge(name)
	if err != nil {
		return err
	}

	backup, err := s.loadBackup(name)
	if err != nil {
		backup = &BridgeBackup{
			BridgeName: name,
			Interfaces: make([]InterfaceBackup, 0),
		}

		if len(bridge.Interfaces) > 0 && bridge.IP != "" {
			backup.Interfaces = append(backup.Interfaces, InterfaceBackup{
				Name:    bridge.Interfaces[0],
				IP:      bridge.IP,
				Netmask: bridge.Netmask,
				Gateway: bridge.Gateway,
				CIDR:    maskToCIDR(bridge.Netmask),
			})
		}
	}

	// Étape 1: Libérer toutes les interfaces du bridge
	for _, ifaceName := range bridge.Interfaces {
		exec.Command("ip", "link", "set", ifaceName, "nomaster").Run()
		exec.Command("ip", "link", "set", ifaceName, "up").Run()
	}

	// Étape 2: Restaurer les IPs sur les interfaces AVANT de supprimer le bridge
	for _, ifaceBackup := range backup.Interfaces {
		if ifaceBackup.IP != "" && ifaceBackup.CIDR > 0 {
			exec.Command("ip", "addr", "add",
				fmt.Sprintf("%s/%d", ifaceBackup.IP, ifaceBackup.CIDR),
				"dev", ifaceBackup.Name).Run()
		}
	}

	// Étape 3: Configurer la gateway sur la première interface qui a une IP
	for _, ifaceBackup := range backup.Interfaces {
		if ifaceBackup.Gateway != "" && ifaceBackup.IP != "" {
			exec.Command("ip", "route", "del", "default").Run()
			exec.Command("ip", "route", "add", "default", "via", ifaceBackup.Gateway, "dev", ifaceBackup.Name).Run()
			break
		}
	}

	// Étape 4: Supprimer le bridge
	exec.Command("ip", "addr", "flush", "dev", name).Run()
	if err := exec.Command("ip", "link", "del", name).Run(); err != nil {
		return fmt.Errorf("erreur suppression bridge: %w", err)
	}

	// Étape 5: Restaurer le fichier interfaces original ou le reconstruire
	if backup.OriginalInterfaces != "" {
		os.WriteFile(interfacesFile, []byte(backup.OriginalInterfaces), 0644)
	} else {
		s.removeBridgeFromInterfaces(name, backup.Interfaces)
	}

	// Étape 6: Nettoyer le backup
	s.deleteBackup(name)

	return nil
}

// ConfigureBridge modifie la configuration IP d'un bridge existant.
func (s *BridgeService) ConfigureBridge(cfg models.Bridge) error {
	existingBridge, err := s.GetBridge(cfg.Name)
	if err != nil {
		return err
	}

	if net.ParseIP(cfg.IP) == nil {
		return fmt.Errorf("adresse IP invalide: %s", cfg.IP)
	}
	if net.ParseIP(cfg.Netmask) == nil {
		return fmt.Errorf("masque invalide: %s", cfg.Netmask)
	}

	exec.Command("ip", "addr", "flush", "dev", cfg.Name).Run()

	cidr := maskToCIDR(cfg.Netmask)
	if err := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", cfg.IP, cidr), "dev", cfg.Name).Run(); err != nil {
		return fmt.Errorf("erreur configuration IP: %w", err)
	}

	if cfg.Gateway != "" {
		exec.Command("ip", "route", "del", "default").Run()
		exec.Command("ip", "route", "add", "default", "via", cfg.Gateway, "dev", cfg.Name).Run()
	}

	if len(cfg.DNS) > 0 {
		s.writeDNS(cfg.DNS)
	}

	cfg.Interfaces = existingBridge.Interfaces
	return s.persistBridgeToInterfaces(cfg)
}

// GetAvailableInterfaces retourne les interfaces disponibles pour un bridge.
func (s *BridgeService) GetAvailableInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var available []string
	for _, iface := range ifaces {
		name := iface.Name

		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.HasPrefix(name, "br") || strings.HasPrefix(name, "lo") {
			continue
		}
		if !strings.HasPrefix(name, "eth") && !strings.HasPrefix(name, "lan") {
			continue
		}
		if s.GetBridgeMaster(name) != "" {
			continue
		}

		available = append(available, name)
	}

	return available, nil
}

// GetBridgeMaster retourne le nom du bridge parent d'une interface.
func (s *BridgeService) GetBridgeMaster(ifaceName string) string {
	masterPath := fmt.Sprintf("/sys/class/net/%s/master", ifaceName)
	if link, err := os.Readlink(masterPath); err == nil {
		return filepath.Base(link)
	}

	out, err := exec.Command("ip", "link", "show", ifaceName).Output()
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`master\s+(\w+)`)
	if matches := re.FindStringSubmatch(string(out)); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// IsInterfaceInBridge vérifie si une interface est esclave d'un bridge.
func (s *BridgeService) IsInterfaceInBridge(ifaceName string) bool {
	return s.GetBridgeMaster(ifaceName) != ""
}

// persistBridgeToInterfaces écrit la config du bridge dans /etc/network/interfaces.
func (s *BridgeService) persistBridgeToInterfaces(cfg models.Bridge) error {
	content, err := os.ReadFile(interfacesFile)
	if err != nil {
		return fmt.Errorf("impossible de lire %s: %w", interfacesFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	skipUntilNextIface := false
	interfacesToSkip := make(map[string]bool)

	for _, iface := range cfg.Interfaces {
		interfacesToSkip[iface] = true
	}
	interfacesToSkip[cfg.Name] = true

	// Parcourir et filtrer les lignes existantes
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Détecter le début d'un bloc iface à supprimer
		if strings.HasPrefix(trimmed, "iface ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				ifaceName := fields[1]
				if interfacesToSkip[ifaceName] {
					skipUntilNextIface = true
					continue
				}
			}
			skipUntilNextIface = false
		}

		// Détecter auto/allow-hotplug pour les interfaces à supprimer
		if strings.HasPrefix(trimmed, "auto ") || strings.HasPrefix(trimmed, "allow-hotplug ") {
			fields := strings.Fields(trimmed)
			allSkipped := true
			var keptIfaces []string
			for _, f := range fields[1:] {
				if !interfacesToSkip[f] {
					allSkipped = false
					keptIfaces = append(keptIfaces, f)
				}
			}
			if allSkipped {
				continue
			}
			if len(keptIfaces) < len(fields)-1 {
				newLines = append(newLines, fields[0]+" "+strings.Join(keptIfaces, " "))
				continue
			}
		}

		if skipUntilNextIface {
			if strings.HasPrefix(trimmed, "iface ") ||
				strings.HasPrefix(trimmed, "auto ") ||
				strings.HasPrefix(trimmed, "allow-hotplug ") ||
				strings.HasPrefix(trimmed, "source") ||
				trimmed == "" {
				if !strings.HasPrefix(trimmed, "iface ") || !interfacesToSkip[strings.Fields(trimmed)[1]] {
					skipUntilNextIface = false
					if trimmed != "" {
						newLines = append(newLines, line)
					}
				}
			}
			continue
		}

		newLines = append(newLines, line)
	}

	// Nettoyer les lignes vides en fin de fichier
	for len(newLines) > 0 && strings.TrimSpace(newLines[len(newLines)-1]) == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Ajouter la configuration du bridge
	newLines = append(newLines, "")
	newLines = append(newLines, "# Bridge configuration managed by moxa-config")

	// Interfaces esclaves en mode manual
	for _, ifaceName := range cfg.Interfaces {
		newLines = append(newLines, fmt.Sprintf("iface %s inet manual", ifaceName))
	}

	// Configuration du bridge avec pre-up
	newLines = append(newLines, "")
	newLines = append(newLines, fmt.Sprintf("auto %s", cfg.Name))
	newLines = append(newLines, fmt.Sprintf("iface %s inet static", cfg.Name))
	newLines = append(newLines, fmt.Sprintf("    pre-up ip link add name %s type bridge || true", cfg.Name))

	for _, ifaceName := range cfg.Interfaces {
		newLines = append(newLines, fmt.Sprintf("    pre-up ip link set %s master %s || true", ifaceName, cfg.Name))
		newLines = append(newLines, fmt.Sprintf("    pre-up ip link set %s up || true", ifaceName))
	}

	newLines = append(newLines, fmt.Sprintf("    address %s", cfg.IP))
	newLines = append(newLines, fmt.Sprintf("    netmask %s", cfg.Netmask))

	if cfg.Gateway != "" {
		newLines = append(newLines, fmt.Sprintf("    gateway %s", cfg.Gateway))
	}

	newLines = append(newLines, "")

	return os.WriteFile(interfacesFile, []byte(strings.Join(newLines, "\n")), 0644)
}

// removeBridgeFromInterfaces supprime la config du bridge et restaure les interfaces.
func (s *BridgeService) removeBridgeFromInterfaces(bridgeName string, interfaces []InterfaceBackup) error {
	content, err := os.ReadFile(interfacesFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	skipUntilNextIface := false
	inMoxaConfigBlock := false

	interfacesToRemove := make(map[string]bool)
	interfacesToRemove[bridgeName] = true
	for _, iface := range interfaces {
		interfacesToRemove[iface.Name] = true
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Détecter le bloc moxa-config
		if strings.Contains(trimmed, "Bridge configuration managed by moxa-config") {
			inMoxaConfigBlock = true
			continue
		}

		if inMoxaConfigBlock {
			// Sortir du bloc si on trouve une autre section non liée
			if strings.HasPrefix(trimmed, "# ") && !strings.Contains(trimmed, "moxa-config") {
				inMoxaConfigBlock = false
			} else if strings.HasPrefix(trimmed, "iface ") {
				fields := strings.Fields(trimmed)
				if len(fields) >= 2 && !interfacesToRemove[fields[1]] {
					inMoxaConfigBlock = false
				} else {
					skipUntilNextIface = true
					continue
				}
			} else if strings.HasPrefix(trimmed, "auto ") {
				fields := strings.Fields(trimmed)
				if len(fields) >= 2 && interfacesToRemove[fields[1]] {
					continue
				}
				inMoxaConfigBlock = false
			} else {
				continue
			}
		}

		if strings.HasPrefix(trimmed, "iface ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 && interfacesToRemove[fields[1]] {
				skipUntilNextIface = true
				continue
			}
			skipUntilNextIface = false
		}

		if skipUntilNextIface {
			if strings.HasPrefix(trimmed, "iface ") ||
				strings.HasPrefix(trimmed, "auto ") ||
				strings.HasPrefix(trimmed, "allow-hotplug ") {
				skipUntilNextIface = false
				if !interfacesToRemove[strings.Fields(trimmed)[1]] {
					newLines = append(newLines, line)
				}
			}
			continue
		}

		newLines = append(newLines, line)
	}

	// Nettoyer les lignes vides multiples
	var cleanedLines []string
	prevEmpty := false
	for _, line := range newLines {
		isEmpty := strings.TrimSpace(line) == ""
		if isEmpty && prevEmpty {
			continue
		}
		cleanedLines = append(cleanedLines, line)
		prevEmpty = isEmpty
	}

	// Restaurer les configs des interfaces si elles avaient des IPs
	for _, iface := range interfaces {
		if iface.IP != "" {
			cleanedLines = append(cleanedLines, "")
			cleanedLines = append(cleanedLines, fmt.Sprintf("auto %s", iface.Name))
			cleanedLines = append(cleanedLines, fmt.Sprintf("iface %s inet static", iface.Name))
			cleanedLines = append(cleanedLines, fmt.Sprintf("    address %s", iface.IP))
			cleanedLines = append(cleanedLines, fmt.Sprintf("    netmask %s", iface.Netmask))
			if iface.Gateway != "" {
				cleanedLines = append(cleanedLines, fmt.Sprintf("    gateway %s", iface.Gateway))
			}
		}
	}

	cleanedLines = append(cleanedLines, "")

	return os.WriteFile(interfacesFile, []byte(strings.Join(cleanedLines, "\n")), 0644)
}

// captureInterfaceConfig lit la config réseau actuelle d'une interface.
func (s *BridgeService) captureInterfaceConfig(ifaceName string) InterfaceBackup {
	backup := InterfaceBackup{Name: ifaceName}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return backup
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return backup
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			backup.IP = ipnet.IP.String()
			backup.Netmask = net.IP(ipnet.Mask).String()
			ones, _ := ipnet.Mask.Size()
			backup.CIDR = ones
			break
		}
	}

	backup.Gateway = s.getGateway(ifaceName)

	return backup
}

// saveBackup sauvegarde la config des interfaces dans un fichier JSON.
func (s *BridgeService) saveBackup(backup BridgeBackup) error {
	if err := os.MkdirAll(s.backupPath, 0755); err != nil {
		return err
	}

	path := filepath.Join(s.backupPath, fmt.Sprintf("%s.bridge.json", backup.BridgeName))
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// loadBackup charge le backup d'un bridge.
func (s *BridgeService) loadBackup(bridgeName string) (*BridgeBackup, error) {
	path := filepath.Join(s.backupPath, fmt.Sprintf("%s.bridge.json", bridgeName))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var backup BridgeBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, err
	}

	return &backup, nil
}

// deleteBackup supprime le fichier de backup d'un bridge.
func (s *BridgeService) deleteBackup(bridgeName string) {
	path := filepath.Join(s.backupPath, fmt.Sprintf("%s.bridge.json", bridgeName))
	os.Remove(path)
}

func (s *BridgeService) getBridgeSlaves(bridgeName string) []string {
	brifPath := fmt.Sprintf("/sys/class/net/%s/brif", bridgeName)
	entries, err := os.ReadDir(brifPath)
	if err != nil {
		return s.getBridgeSlavesFromIP(bridgeName)
	}

	var slaves []string
	for _, entry := range entries {
		slaves = append(slaves, entry.Name())
	}

	return slaves
}

func (s *BridgeService) getBridgeSlavesFromIP(bridgeName string) []string {
	var slaves []string

	out, err := exec.Command("ip", "link", "show").Output()
	if err != nil {
		return slaves
	}

	re := regexp.MustCompile(`\d+:\s+(\w+):.*master\s+` + regexp.QuoteMeta(bridgeName))
	matches := re.FindAllStringSubmatch(string(out), -1)
	for _, match := range matches {
		if len(match) > 1 {
			slaves = append(slaves, match[1])
		}
	}

	return slaves
}

func (s *BridgeService) validateBridgeConfig(cfg models.Bridge) error {
	if cfg.Name == "" {
		return fmt.Errorf("nom du bridge requis")
	}
	if !strings.HasPrefix(cfg.Name, "br") {
		return fmt.Errorf("le nom du bridge doit commencer par 'br'")
	}
	if len(cfg.Interfaces) < 1 {
		return fmt.Errorf("au moins une interface requise")
	}
	if net.ParseIP(cfg.IP) == nil {
		return fmt.Errorf("adresse IP invalide: %s", cfg.IP)
	}
	if net.ParseIP(cfg.Netmask) == nil {
		return fmt.Errorf("masque invalide: %s", cfg.Netmask)
	}
	if cfg.Gateway != "" && net.ParseIP(cfg.Gateway) == nil {
		return fmt.Errorf("passerelle invalide: %s", cfg.Gateway)
	}

	for _, ifaceName := range cfg.Interfaces {
		if _, err := net.InterfaceByName(ifaceName); err != nil {
			return fmt.Errorf("interface %s non trouvée", ifaceName)
		}
	}

	return nil
}

func (s *BridgeService) getStatus(flags net.Flags) string {
	if flags&net.FlagUp != 0 {
		return "up"
	}
	return "down"
}

func (s *BridgeService) getGateway(ifaceName string) string {
	out, err := exec.Command("ip", "route", "show", "dev", ifaceName).Output()
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`default via (\d+\.\d+\.\d+\.\d+)`)
	if matches := re.FindStringSubmatch(string(out)); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (s *BridgeService) readDNS() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}

	var servers []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "nameserver ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				servers = append(servers, parts[1])
			}
		}
	}
	return servers
}

func (s *BridgeService) writeDNS(servers []string) error {
	var content strings.Builder
	content.WriteString("# Generated by moxa-config\n")
	for _, dns := range servers {
		content.WriteString(fmt.Sprintf("nameserver %s\n", dns))
	}
	return os.WriteFile("/etc/resolv.conf", []byte(content.String()), 0644)
}
