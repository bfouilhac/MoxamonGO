// Package auth gère l'authentification et les sessions.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User représente un utilisateur.
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	MustChange   bool   `json:"mustChange"`
}

// Session représente une session active.
type Session struct {
	Username  string
	ExpiresAt time.Time
}

// AuthService gère l'authentification.
type AuthService struct {
	usersFile    string
	users        map[string]*User
	sessions     map[string]*Session
	sessionMutex sync.RWMutex
	usersMutex   sync.RWMutex
}

// NewAuthService crée un nouveau service d'authentification.
func NewAuthService(usersFile string) (*AuthService, error) {
	s := &AuthService{
		usersFile: usersFile,
		users:     make(map[string]*User),
		sessions:  make(map[string]*Session),
	}

	if err := s.loadUsers(); err != nil {
		if os.IsNotExist(err) {
			if err := s.createDefaultAdmin(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	go s.cleanupExpiredSessions()

	return s, nil
}

func (s *AuthService) loadUsers() error {
	data, err := os.ReadFile(s.usersFile)
	if err != nil {
		return err
	}

	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	s.usersMutex.Lock()
	defer s.usersMutex.Unlock()

	for _, u := range users {
		s.users[u.Username] = u
	}

	return nil
}

func (s *AuthService) saveUsers() error {
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.usersFile, data, 0600)
}

func (s *AuthService) createDefaultAdmin() error {
	hash, err := bcrypt.GenerateFromPassword([]byte("moxa"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.users["moxa"] = &User{
		Username:     "moxa",
		PasswordHash: string(hash),
		MustChange:   true,
	}

	return s.saveUsers()
}

// Authenticate vérifie les identifiants et crée une session.
func (s *AuthService) Authenticate(username, password string) (string, *User, error) {
	s.usersMutex.RLock()
	user, exists := s.users[username]
	s.usersMutex.RUnlock()

	if !exists {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return "", nil, err
	}

	s.sessionMutex.Lock()
	s.sessions[token] = &Session{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	s.sessionMutex.Unlock()

	return token, user, nil
}

// ValidateSession vérifie si une session est valide.
func (s *AuthService) ValidateSession(token string) (*Session, bool) {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	session, exists := s.sessions[token]
	if !exists || time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// Logout supprime une session.
func (s *AuthService) Logout(token string) {
	s.sessionMutex.Lock()
	delete(s.sessions, token)
	s.sessionMutex.Unlock()
}

// ChangePassword change le mot de passe d'un utilisateur.
func (s *AuthService) ChangePassword(username, newPassword string) error {
	log.Printf("AuthService.ChangePassword: starting for user %s", username)

	s.usersMutex.Lock()
	defer s.usersMutex.Unlock()

	log.Printf("AuthService.ChangePassword: mutex acquired")

	user, exists := s.users[username]
	if !exists {
		log.Printf("AuthService.ChangePassword: user not found")
		return ErrUserNotFound
	}
	log.Printf("AuthService.ChangePassword: user found")

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("AuthService.ChangePassword: bcrypt error - %v", err)
		return err
	}
	log.Printf("AuthService.ChangePassword: hash generated")

	user.PasswordHash = string(hash)
	user.MustChange = false

	log.Printf("AuthService.ChangePassword: saving users")
	err = s.saveUsers()
	if err != nil {
		log.Printf("AuthService.ChangePassword: save error - %v", err)
	} else {
		log.Printf("AuthService.ChangePassword: save success")
	}
	return err
}

// Middleware protège les routes authentifiées.
func (s *AuthService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, `{"success":false,"message":"Non authentifié"}`, http.StatusUnauthorized)
			return
		}

		if _, valid := s.ValidateSession(cookie.Value); !valid {
			http.Error(w, `{"success":false,"message":"Session expirée"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *AuthService) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		s.sessionMutex.Lock()
		now := time.Now()
		for token, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, token)
			}
		}
		s.sessionMutex.Unlock()
	}
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Erreurs
var (
	ErrInvalidCredentials = &AuthError{"Identifiants invalides"}
	ErrUserNotFound       = &AuthError{"Utilisateur non trouvé"}
)

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
