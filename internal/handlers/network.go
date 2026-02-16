// Package handlers implémente les endpoints HTTP de l'API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"moxa-config/internal/models"
	"moxa-config/internal/services"
)

// NetworkHandler gère les requêtes HTTP pour la configuration réseau.
type NetworkHandler struct {
	service *services.NetworkService
}

// NewNetworkHandler crée une instance du handler réseau.
func NewNetworkHandler(svc *services.NetworkService) *NetworkHandler {
	return &NetworkHandler{service: svc}
}

// ServeHTTP route les requêtes vers les méthodes appropriées.
func (h *NetworkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/network")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if path == "" || path == "interfaces" {
			h.list(w, r)
		} else if strings.HasPrefix(path, "interface/") {
			h.get(w, r, strings.TrimPrefix(path, "interface/"))
		} else if path == "backups" {
			h.listBackups(w, r)
		} else {
			http.NotFound(w, r)
		}

	case http.MethodPost:
		if path == "configure" {
			h.configure(w, r)
		} else if path == "test" {
			h.testConfig(w, r)
		} else if strings.HasPrefix(path, "restore/") {
			h.restore(w, r, strings.TrimPrefix(path, "restore/"))
		} else {
			http.NotFound(w, r)
		}

	default:
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *NetworkHandler) list(w http.ResponseWriter, _ *http.Request) {
	ifaces, err := h.service.ListInterfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, ifaces)
}

func (h *NetworkHandler) get(w http.ResponseWriter, _ *http.Request, name string) {
	cfg, err := h.service.GetInterfaceConfig(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, cfg)
}

func (h *NetworkHandler) configure(w http.ResponseWriter, r *http.Request) {
	var cfg models.NetworkInterface
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide: "+err.Error())
		return
	}

	if err := h.service.Configure(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Configuration appliquée avec succès")
}

func (h *NetworkHandler) testConfig(w http.ResponseWriter, r *http.Request) {
	var cfg models.NetworkInterface
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide: "+err.Error())
		return
	}

	if err := h.service.ApplyWithoutPersist(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Configuration test appliquée (non persistée)")
}

func (h *NetworkHandler) restore(w http.ResponseWriter, _ *http.Request, name string) {
	if err := h.service.RestoreBackup(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "Backup restauré avec succès")
}

func (h *NetworkHandler) listBackups(w http.ResponseWriter, _ *http.Request) {
	backups, err := h.service.ListBackups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, backups)
}
