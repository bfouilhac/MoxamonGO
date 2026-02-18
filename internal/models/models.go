// Package models définit les structures de données pour la configuration MOXA.
package models

// NetworkInterface représente la configuration d'une interface réseau.
type NetworkInterface struct {
	Name         string   `json:"name"`
	IP           string   `json:"ip"`
	Netmask      string   `json:"netmask"`
	Gateway      string   `json:"gateway,omitempty"`
	DNS          []string `json:"dns,omitempty"`
	DHCP         bool     `json:"dhcp"`
	Status       string   `json:"status,omitempty"`
	MAC          string   `json:"mac,omitempty"`
	BridgeMaster string   `json:"bridgeMaster,omitempty"`
}

// Bridge représente la configuration d'un bridge réseau.
type Bridge struct {
	Name       string   `json:"name"`
	Interfaces []string `json:"interfaces"`
	IP         string   `json:"ip"`
	Netmask    string   `json:"netmask"`
	Gateway    string   `json:"gateway,omitempty"`
	DNS        []string `json:"dns,omitempty"`
	Status     string   `json:"status"`
	MAC        string   `json:"mac,omitempty"`
}

// SerialPort représente la configuration d'un port série.
type SerialPort struct {
	Device   string `json:"device"`
	BaudRate int    `json:"baudRate"`
	DataBits int    `json:"dataBits"`
	StopBits int    `json:"stopBits"`
	Parity   string `json:"parity"`
	FlowCtrl string `json:"flowControl"`
	Mode     string `json:"mode"`
}

// APIResponse encapsule les réponses JSON de l'API.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// ParityOptions définit les options de parité disponibles.
var ParityOptions = []string{"none", "odd", "even", "mark", "space"}

// FlowControlOptions définit les options de contrôle de flux.
var FlowControlOptions = []string{"none", "hardware", "software"}

// SerialModeOptions définit les modes de communication série.
var SerialModeOptions = []string{"RS232", "RS422", "RS485-2W", "RS485-4W"}

// StandardBaudRates liste les débits série standards.
var StandardBaudRates = []int{300, 1200, 2400, 4800, 9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}
