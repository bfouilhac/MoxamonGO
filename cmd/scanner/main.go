// Scanner de découverte des UC MOXA sur le réseau.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"time"
)

const (
	DiscoveryPort    = 54321
	DiscoveryRequest = "MOXA_DISCOVER"
	ScanTimeout      = 3 * time.Second
)

type DeviceInfo struct {
	ProductName string `json:"productName"`
	IPAddress   string `json:"ipAddress"`
	Version     string `json:"version,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
}

func main() {
	fmt.Println("MoxamonGO Scanner")
	fmt.Println("Recherche des UC sur le réseau...")
	fmt.Println()

	devices, err := scan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur: %v\n", err)
		os.Exit(1)
	}

	if len(devices) == 0 {
		fmt.Println("Aucun UC trouvé.")
		return
	}

	sort.Slice(devices, func(i, j int) bool { return devices[i].IPAddress < devices[j].IPAddress })

	printTable(devices)
}

func scan() ([]DeviceInfo, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("impossible d'ouvrir le socket UDP: %w", err)
	}
	defer conn.Close()

	// Envoyer en broadcast global (255.255.255.255)
	broadcastAddr := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: DiscoveryPort,
	}
	if _, err := conn.WriteToUDP([]byte(DiscoveryRequest), broadcastAddr); err != nil {
		return nil, fmt.Errorf("impossible d'envoyer le broadcast: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(ScanTimeout))

	devices := make(map[string]DeviceInfo)
	buf := make([]byte, 1024)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		var info DeviceInfo
		if err := json.Unmarshal(buf[:n], &info); err != nil {
			continue
		}

		// Utiliser l'IP source si l'IP dans la réponse est vide ou différente
		if info.IPAddress == "" {
			info.IPAddress = remoteAddr.IP.String()
		}

		devices[info.IPAddress] = info
	}

	result := make([]DeviceInfo, 0, len(devices))
	for _, d := range devices {
		result = append(result, d)
	}

	return result, nil
}

func printTable(devices []DeviceInfo) {
	maxProduct := 12
	maxIP := 15

	for _, d := range devices {
		if len(d.ProductName) > maxProduct {
			maxProduct = len(d.ProductName)
		}
		if len(d.IPAddress) > maxIP {
			maxIP = len(d.IPAddress)
		}
	}

	separator := "+" + repeat("-", maxProduct+2) + "+" + repeat("-", maxIP+2) + "+"
	fmt.Println(separator)
	fmt.Printf("| %-*s | %-*s |\n", maxProduct, "Produit", maxIP, "Adresse IP")
	fmt.Println(separator)

	for _, d := range devices {
		fmt.Printf("| %-*s | %-*s |\n", maxProduct, d.ProductName, maxIP, d.IPAddress)
	}

	fmt.Println(separator)
	fmt.Printf("\n%d UC trouvé(s)\n", len(devices))
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
