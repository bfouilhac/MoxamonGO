// Package handlers implémente les endpoints HTTP de l'API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"moxa-config/internal/models"
	"moxa-config/internal/services"
)

// BridgeHandler gère les requêtes HTTP pour la configuration des bridges.
type BridgeHandler struct {
	service *services.BridgeService
}

// NewBridgeHandler crée une instance du handler bridge.
func NewBridgeHandler(svc *services.BridgeService) *BridgeHandler {
	return &BridgeHandler{service: svc}
}

// ServeHTTP route les requêtes vers les méthodes appropriées.
func (h *BridgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/bridge")
	path = strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if path == "" || path == "list" {
			h.list(w, r)
		} else if path == "available" {
			h.available(w, r)
		} else {
			h.get(w, r, path)
		}

	case http.MethodPost:
		if path == "create" {
			h.create(w, r)
		} else if path == "configure" {
			h.configure(w, r)
		} else if strings.HasPrefix(path, "delete/") {
			h.delete(w, r, strings.TrimPrefix(path, "delete/"))
		} else {
			http.NotFound(w, r)
		}

	default:
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *BridgeHandler) list(w http.ResponseWriter, _ *http.Request) {
	bridges, err := h.service.ListBridges()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, bridges)
}

func (h *BridgeHandler) get(w http.ResponseWriter, _ *http.Request, name string) {
	bridge, err := h.service.GetBridge(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, bridge)
}

func (h *BridgeHandler) available(w http.ResponseWriter, _ *http.Request) {
	ifaces, err := h.service.GetAvailableInterfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, ifaces)
}

func (h *BridgeHandler) create(w http.ResponseWriter, r *http.Request) {
	var cfg models.Bridge
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide: "+err.Error())
		return
	}

	if err := h.service.CreateBridge(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Bridge créé avec succès")
}

func (h *BridgeHandler) configure(w http.ResponseWriter, r *http.Request) {
	var cfg models.Bridge
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalide: "+err.Error())
		return
	}

	if err := h.service.ConfigureBridge(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Bridge configuré avec succès")
}

func (h *BridgeHandler) delete(w http.ResponseWriter, _ *http.Request, name string) {
	if err := h.service.DeleteBridge(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeMessage(w, "Bridge supprimé avec succès")
}
