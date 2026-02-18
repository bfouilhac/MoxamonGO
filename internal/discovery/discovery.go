// Package discovery gère la découverte des UC sur le réseau.
package discovery

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

const (
	DiscoveryPort    = 54321
	DiscoveryRequest = "MOXA_DISCOVER"
	MaxPacketSize    = 1024
)

// DeviceInfo contient les informations d'un UC.
type DeviceInfo struct {
	ProductName string `json:"productName"`
	IPAddress   string `json:"ipAddress"`
	Version     string `json:"version,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
}

// Service gère l'écoute des requêtes de découverte.
type Service struct {
	conn   *net.UDPConn
	info   DeviceInfo
	stopCh chan struct{}
}

// NewService crée un nouveau service de découverte.
func NewService() (*Service, error) {
	info, err := getDeviceInfo()
	if err != nil {
		return nil, err
	}

	return &Service{
		info:   info,
		stopCh: make(chan struct{}),
	}, nil
}

// Start démarre le service d'écoute UDP.
func (s *Service) Start() error {
	addr := &net.UDPAddr{
		Port: DiscoveryPort,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}

	s.conn = conn
	go s.listen()

	log.Printf("Discovery service listening on port %d", DiscoveryPort)
	return nil
}

// Stop arrête le service.
func (s *Service) Stop() {
	close(s.stopCh)
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *Service) listen() {
	buf := make([]byte, MaxPacketSize)

	for {
		select {
		case <-s.stopCh:
			return
		default:
			n, remoteAddr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			request := strings.TrimSpace(string(buf[:n]))
			if request == DiscoveryRequest {
				s.respond(remoteAddr)
			}
		}
	}
}

func (s *Service) respond(addr *net.UDPAddr) {
	s.info.IPAddress = getLocalIPForDiscovery()

	response, err := json.Marshal(s.info)
	if err != nil {
		log.Printf("Discovery: erreur encodage réponse: %v", err)
		return
	}

	// Répondre en unicast (fonctionne pour le même sous-réseau)
	s.conn.WriteToUDP(response, addr)

	// Aussi répondre en broadcast pour les scanners sur d'autres sous-réseaux
	// Utiliser un socket dédié car le socket principal peut ne pas avoir SO_BROADCAST
	broadcastConn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: addr.Port,
	})
	if err == nil {
		broadcastConn.Write(response)
		broadcastConn.Close()
	}
}

func getDeviceInfo() (DeviceInfo, error) {
	info := DeviceInfo{}

	out, err := exec.Command("mx-ver").Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
		parts := strings.SplitN(line, " version ", 2)
		if len(parts) >= 1 {
			info.ProductName = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			versionParts := strings.Fields(parts[1])
			if len(versionParts) >= 1 {
				info.Version = versionParts[0]
			}
		}
	} else {
		info.ProductName = "MOXA UC (unknown)"
	}

	hostname, err := os.Hostname()
	if err == nil {
		info.Hostname = hostname
	}

	return info, nil
}

func getLocalIPForDiscovery() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	// Chercher la première IP non-loopback (préférer eth0, br0)
	var fallbackIP string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				// Préférer br0 ou eth0
				if strings.HasPrefix(iface.Name, "br") || strings.HasPrefix(iface.Name, "eth") {
					return ip
				}
				if fallbackIP == "" {
					fallbackIP = ip
				}
			}
		}
	}

	return fallbackIP
}
