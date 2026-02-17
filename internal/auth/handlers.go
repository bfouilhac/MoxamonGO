package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Handler gère les requêtes d'authentification.
type Handler struct {
	service *AuthService
	secure  bool
}

// NewHandler crée un nouveau handler d'authentification.
func NewHandler(service *AuthService, secure bool) *Handler {
	return &Handler{service: service, secure: secure}
}

// Login gère la connexion.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login attempt from %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("Login: wrong method %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Requête invalide",
		})
		return
	}

	token, user, err := h.service.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"success": false,
			"message": "Identifiants invalides",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"mustChange": user.MustChange,
		"username":   user.Username,
	})
}

// Logout gère la déconnexion.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.service.Logout(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Déconnecté",
	})
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	log.Printf("ChangePassword attempt from %s", r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Printf("ChangePassword: wrong method %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil {
		log.Printf("ChangePassword: no cookie - %v", err)
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"success": false,
			"message": "Non authentifié",
		})
		return
	}
	log.Printf("ChangePassword: cookie found")

	session, valid := h.service.ValidateSession(cookie.Value)
	if !valid {
		log.Printf("ChangePassword: invalid session")
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"success": false,
			"message": "Session expirée",
		})
		return
	}
	log.Printf("ChangePassword: session valid for user %s", session.Username)

	var req struct {
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ChangePassword: decode error - %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Requête invalide",
		})
		return
	}
	log.Printf("ChangePassword: new password length %d", len(req.NewPassword))

	if len(req.NewPassword) < 8 {
		log.Printf("ChangePassword: password too short")
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"message": "Le mot de passe doit contenir au moins 8 caractères",
		})
		return
	}

	if err := h.service.ChangePassword(session.Username, req.NewPassword); err != nil {
		log.Printf("ChangePassword: service error - %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": "Erreur lors du changement de mot de passe",
		})
		return
	}

	log.Printf("ChangePassword: success for user %s", session.Username)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Mot de passe modifié",
	})
}

// CheckSession vérifie si l'utilisateur est connecté.
func (h *Handler) CheckSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
		})
		return
	}

	session, valid := h.service.ValidateSession(cookie.Value)
	if !valid {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
		})
		return
	}

	h.service.usersMutex.RLock()
	user := h.service.users[session.Username]
	mustChange := user != nil && user.MustChange
	h.service.usersMutex.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      session.Username,
		"mustChange":    mustChange,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
