package services

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"moxa-config/internal/models"
)

// SerialService gère les opérations sur les ports série.
type SerialService struct {
	configPath string
}

// NewSerialService crée une instance du service série.
func NewSerialService(configPath string) *SerialService {
	return &SerialService{configPath: configPath}
}

// ListPorts retourne les ports série physiques MOXA (ttyM0 et ttyM1).
func (s *SerialService) ListPorts() ([]models.SerialPort, error) {
	moxaPorts := []string{"/dev/ttyM0", "/dev/ttyM1"}
	var ports []models.SerialPort

	for _, dev := range moxaPorts {
		if _, err := os.Stat(dev); os.IsNotExist(err) {
			continue
		}

		port := models.SerialPort{
			Device:   dev,
			BaudRate: 9600,
			DataBits: 8,
			StopBits: 1,
			Parity:   "none",
			FlowCtrl: "none",
			Mode:     "RS232",
		}

		if cfg, err := s.readCurrentConfig(dev); err == nil {
			port = *cfg
		}

		ports = append(ports, port)
	}

	return ports, nil
}

// Configure applique une configuration à un port série.
func (s *SerialService) Configure(cfg models.SerialPort) error {
	if err := s.validateConfig(cfg); err != nil {
		return err
	}

	if err := s.backupConfig(cfg.Device); err != nil {
		return fmt.Errorf("échec backup: %w", err)
	}

	if cfg.Mode != "" {
		if err := s.applyMode(cfg.Device, cfg.Mode); err != nil {
			return fmt.Errorf("échec application mode: %w", err)
		}
	}

	if err := s.applyConfig(cfg); err != nil {
		return fmt.Errorf("échec application config: %w", err)
	}

	return s.persistConfig(cfg)
}

// applyMode configure le mode RS232/422/485 via setinterface MOXA.
func (s *SerialService) applyMode(device, mode string) error {
	portNum := "0"
	if strings.HasSuffix(device, "1") {
		portNum = "1"
	}

	modeArg := ""
	switch mode {
	case "RS232":
		modeArg = "rs232"
	case "RS422":
		modeArg = "rs422"
	case "RS485-2W":
		modeArg = "rs485-2w"
	case "RS485-4W":
		modeArg = "rs485-4w"
	default:
		return fmt.Errorf("mode inconnu: %s", mode)
	}

	cmd := exec.Command("setinterface", portNum, modeArg)
	if out, err := cmd.CombinedOutput(); err != nil {
		cmd = exec.Command("mx-set-serial-mode", portNum, modeArg)
		if out, err = cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("setinterface error: %s - %w", string(out), err)
		}
	}
	return nil
}

func (s *SerialService) validateConfig(cfg models.SerialPort) error {
	if cfg.Device == "" {
		return fmt.Errorf("device requis")
	}

	if _, err := os.Stat(cfg.Device); os.IsNotExist(err) {
		return fmt.Errorf("device introuvable: %s", cfg.Device)
	}

	validBaud := false
	for _, b := range models.StandardBaudRates {
		if cfg.BaudRate == b {
			validBaud = true
			break
		}
	}
	if !validBaud {
		return fmt.Errorf("baudrate invalide: %d", cfg.BaudRate)
	}

	if cfg.DataBits < 5 || cfg.DataBits > 8 {
		return fmt.Errorf("databits invalide: %d (5-8)", cfg.DataBits)
	}

	if cfg.StopBits < 1 || cfg.StopBits > 2 {
		return fmt.Errorf("stopbits invalide: %d (1-2)", cfg.StopBits)
	}

	validParity := false
	for _, p := range models.ParityOptions {
		if cfg.Parity == p {
			validParity = true
			break
		}
	}
	if !validParity {
		return fmt.Errorf("parité invalide: %s", cfg.Parity)
	}

	validFlow := false
	for _, f := range models.FlowControlOptions {
		if cfg.FlowCtrl == f {
			validFlow = true
			break
		}
	}
	if !validFlow {
		return fmt.Errorf("flow control invalide: %s", cfg.FlowCtrl)
	}

	if cfg.Mode != "" {
		validMode := false
		for _, m := range models.SerialModeOptions {
			if cfg.Mode == m {
				validMode = true
				break
			}
		}
		if !validMode {
			return fmt.Errorf("mode invalide: %s", cfg.Mode)
		}
	}

	return nil
}

func (s *SerialService) applyConfig(cfg models.SerialPort) error {
	args := []string{"-F", cfg.Device}
	args = append(args, strconv.Itoa(cfg.BaudRate))
	args = append(args, fmt.Sprintf("cs%d", cfg.DataBits))

	switch cfg.StopBits {
	case 1:
		args = append(args, "-cstopb")
	case 2:
		args = append(args, "cstopb")
	}

	switch cfg.Parity {
	case "none":
		args = append(args, "-parenb")
	case "even":
		args = append(args, "parenb", "-parodd")
	case "odd":
		args = append(args, "parenb", "parodd")
	}

	switch cfg.FlowCtrl {
	case "none":
		args = append(args, "-crtscts", "-ixon", "-ixoff")
	case "hardware":
		args = append(args, "crtscts", "-ixon", "-ixoff")
	case "software":
		args = append(args, "-crtscts", "ixon", "ixoff")
	}

	args = append(args, "raw", "-echo")

	cmd := exec.Command("stty", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stty error: %s - %w", string(out), err)
	}

	return nil
}

func (s *SerialService) persistConfig(cfg models.SerialPort) error {
	deviceName := filepath.Base(cfg.Device)
	path := fmt.Sprintf("%s/serial_%s.conf", s.configPath, deviceName)

	content := fmt.Sprintf(`DEVICE=%s
BAUDRATE=%d
DATABITS=%d
STOPBITS=%d
PARITY=%s
FLOWCONTROL=%s
MODE=%s
`, cfg.Device, cfg.BaudRate, cfg.DataBits, cfg.StopBits, cfg.Parity, cfg.FlowCtrl, cfg.Mode)

	return os.WriteFile(path, []byte(content), 0644)
}

func (s *SerialService) backupConfig(device string) error {
	deviceName := filepath.Base(device)
	src := fmt.Sprintf("%s/serial_%s.conf", s.configPath, deviceName)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	dst := fmt.Sprintf("%s/serial_%s.conf.bak", s.configPath, deviceName)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (s *SerialService) readCurrentConfig(device string) (*models.SerialPort, error) {
	deviceName := filepath.Base(device)
	path := fmt.Sprintf("%s/serial_%s.conf", s.configPath, deviceName)

	file, err := os.Open(path)
	if err != nil {
		return s.readFromStty(device)
	}
	defer file.Close()

	cfg := &models.SerialPort{Device: device}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, val := parts[0], parts[1]
		switch key {
		case "BAUDRATE":
			cfg.BaudRate, _ = strconv.Atoi(val)
		case "DATABITS":
			cfg.DataBits, _ = strconv.Atoi(val)
		case "STOPBITS":
			cfg.StopBits, _ = strconv.Atoi(val)
		case "PARITY":
			cfg.Parity = val
		case "FLOWCONTROL":
			cfg.FlowCtrl = val
		case "MODE":
			cfg.Mode = val
		}
	}

	if cfg.Mode == "" {
		cfg.Mode = s.readCurrentMode(device)
	}

	return cfg, nil
}

// readCurrentMode lit le mode actuel via getinterface MOXA.
func (s *SerialService) readCurrentMode(device string) string {
	portNum := "0"
	if strings.HasSuffix(device, "1") {
		portNum = "1"
	}

	out, err := exec.Command("getinterface", portNum).Output()
	if err != nil {
		out, err = exec.Command("mx-get-serial-mode", portNum).Output()
		if err != nil {
			return "RS232"
		}
	}

	output := strings.ToLower(strings.TrimSpace(string(out)))
	switch {
	case strings.Contains(output, "rs485-2w") || strings.Contains(output, "rs485_2w"):
		return "RS485-2W"
	case strings.Contains(output, "rs485-4w") || strings.Contains(output, "rs485_4w"):
		return "RS485-4W"
	case strings.Contains(output, "rs485"):
		return "RS485-2W"
	case strings.Contains(output, "rs422"):
		return "RS422"
	default:
		return "RS232"
	}
}

func (s *SerialService) readFromStty(device string) (*models.SerialPort, error) {
	out, err := exec.Command("stty", "-F", device, "-a").Output()
	if err != nil {
		return nil, err
	}

	cfg := &models.SerialPort{
		Device:   device,
		BaudRate: 9600,
		DataBits: 8,
		StopBits: 1,
		Parity:   "none",
		FlowCtrl: "none",
	}

	output := string(out)

	if re := regexp.MustCompile(`speed (\d+) baud`); true {
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			cfg.BaudRate, _ = strconv.Atoi(matches[1])
		}
	}

	if re := regexp.MustCompile(`cs(\d)`); true {
		if matches := re.FindStringSubmatch(output); len(matches) > 1 {
			cfg.DataBits, _ = strconv.Atoi(matches[1])
		}
	}

	if strings.Contains(output, " cstopb") && !strings.Contains(output, "-cstopb") {
		cfg.StopBits = 2
	}

	if strings.Contains(output, " parenb") && !strings.Contains(output, "-parenb") {
		if strings.Contains(output, " parodd") && !strings.Contains(output, "-parodd") {
			cfg.Parity = "odd"
		} else {
			cfg.Parity = "even"
		}
	}

	if strings.Contains(output, " crtscts") && !strings.Contains(output, "-crtscts") {
		cfg.FlowCtrl = "hardware"
	} else if strings.Contains(output, " ixon") && !strings.Contains(output, "-ixon") {
		cfg.FlowCtrl = "software"
	}

	return cfg, nil
}

func (s *SerialService) isValidSerialPort(device string) bool {
	info, err := os.Stat(device)
	if err != nil {
		return false
	}

	mode := info.Mode()
	return mode&os.ModeCharDevice != 0
}

// GetPortConfig retourne la configuration d'un port spécifique.
func (s *SerialService) GetPortConfig(device string) (*models.SerialPort, error) {
	if _, err := os.Stat(device); os.IsNotExist(err) {
		return nil, fmt.Errorf("device introuvable: %s", device)
	}
	return s.readCurrentConfig(device)
}

// RestoreBackup restaure une configuration série sauvegardée.
func (s *SerialService) RestoreBackup(device string) error {
	deviceName := filepath.Base(device)
	src := fmt.Sprintf("%s/serial_%s.conf.bak", s.configPath, deviceName)

	cfg, err := s.readConfigFile(src)
	if err != nil {
		return fmt.Errorf("backup introuvable: %w", err)
	}

	return s.Configure(*cfg)
}

func (s *SerialService) readConfigFile(path string) (*models.SerialPort, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &models.SerialPort{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, val := parts[0], parts[1]
		switch key {
		case "DEVICE":
			cfg.Device = val
		case "BAUDRATE":
			cfg.BaudRate, _ = strconv.Atoi(val)
		case "DATABITS":
			cfg.DataBits, _ = strconv.Atoi(val)
		case "STOPBITS":
			cfg.StopBits, _ = strconv.Atoi(val)
		case "PARITY":
			cfg.Parity = val
		case "FLOWCONTROL":
			cfg.FlowCtrl = val
		}
	}

	return cfg, scanner.Err()
}

// TestPort effectue un test de loopback sur un port série.
func (s *SerialService) TestPort(device string) error {
	file, err := os.OpenFile(device, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("impossible d'ouvrir %s: %w", device, err)
	}
	defer file.Close()

	testData := []byte("MOXA_TEST")
	_, err = file.Write(testData)
	if err != nil {
		return fmt.Errorf("erreur écriture: %w", err)
	}

	return nil
}

// ApplyAtBoot génère le script de démarrage pour les ports série.
func (s *SerialService) ApplyAtBoot() error {
	ports, err := s.ListPorts()
	if err != nil {
		return err
	}

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("# Auto-generated by moxa-config\n\n")

	for _, port := range ports {
		args := s.buildSttyArgs(port)
		script.WriteString(fmt.Sprintf("stty -F %s %s\n", port.Device, strings.Join(args, " ")))
	}

	return os.WriteFile("/etc/init.d/serial-config", []byte(script.String()), 0755)
}

func (s *SerialService) buildSttyArgs(cfg models.SerialPort) []string {
	var args []string

	args = append(args, strconv.Itoa(cfg.BaudRate))
	args = append(args, fmt.Sprintf("cs%d", cfg.DataBits))

	if cfg.StopBits == 2 {
		args = append(args, "cstopb")
	} else {
		args = append(args, "-cstopb")
	}

	switch cfg.Parity {
	case "none":
		args = append(args, "-parenb")
	case "even":
		args = append(args, "parenb", "-parodd")
	case "odd":
		args = append(args, "parenb", "parodd")
	}

	switch cfg.FlowCtrl {
	case "hardware":
		args = append(args, "crtscts")
	case "software":
		args = append(args, "ixon", "ixoff")
	default:
		args = append(args, "-crtscts", "-ixon", "-ixoff")
	}

	return args
}
