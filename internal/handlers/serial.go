package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"moxa-config/internal/models"
	"moxa-config/internal/services"
)

// SerialHandler gère les requêtes HTTP pour la configuration série.
type SerialHandler struct {
	service *services.SerialService
}

// NewSerialHandler crée une instance du handler série.
func NewSerialHandler(svc *services.SerialService) *SerialHandler {
	return &SerialHandler{service: svc}
}

// ServeHTTP route les requêtes vers les méthodes appropriées.
func (h *SerialHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/serial")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if path == "" || path == "ports" {
			h.list(w, r)
		} else if strings.HasPrefix(path, "port/") {
			device, _ := url.PathUnescape(strings.TrimPrefix(path, "port/"))
			h.get(w, r, device)
		} else if path == "options" {
			h.options(w, r)
		} else {
			http.NotFound(w, r)
		}

	case http.MethodPost:
		if path == "configure" {
			h.configure(w, r)
		} else if strings.HasPrefix(path, "test/") {
			device, _ := url.PathUnescape(strings.TrimPrefix(path, "test/"))
			h.test(w, r, device)
		} else if strings.HasPrefix(path, "restore/") {
			device, _ := url.PathUnescape(strings.TrimPrefix(path, "restore/"))
			h.restore(w, r, device)
		} else {
			http.NotFound(w, r)
		}

	default:
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SerialHandler) list(w http.ResponseWriter, _ *http.Request) {
	ports, err := h.service.ListPorts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, ports)
}

func (h *SerialHandler) get(w http.ResponseWriter, _ *http.Request, device string) {
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + device
	}

	cfg, err := h.service.GetPortConfig(device)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, cfg)
}

func (h *SerialHandler) configure(w http.ResponseWriter, r *http.Request) {
	var cfg models.SerialPort
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide: "+err.Error())
		return
	}

	if err := h.service.Configure(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Configuration série appliquée avec succès")
}

func (h *SerialHandler) test(w http.ResponseWriter, _ *http.Request, device string) {
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + device
	}

	if err := h.service.TestPort(device); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Test port réussi")
}

func (h *SerialHandler) restore(w http.ResponseWriter, _ *http.Request, device string) {
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + device
	}

	if err := h.service.RestoreBackup(device); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Configuration série restaurée")
}

func (h *SerialHandler) options(w http.ResponseWriter, _ *http.Request) {
	opts := map[string]any{
		"baudRates":   models.StandardBaudRates,
		"parities":    models.ParityOptions,
		"flowControl": models.FlowControlOptions,
		"modes":       models.SerialModeOptions,
		"dataBits":    []int{5, 6, 7, 8},
		"stopBits":    []int{1, 2},
	}
	writeSuccess(w, opts)
}

func writeSuccess(w http.ResponseWriter, data any) {
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Data:    data,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: false,
		Message: message,
	})
}

func writeMessage(w http.ResponseWriter, message string) {
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: message,
	})
}
