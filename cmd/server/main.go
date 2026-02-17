// Package main lance le serveur de configuration MOXA.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
	"encoding/pem"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"moxa-config/internal/auth"
	"moxa-config/internal/config"
	"moxa-config/internal/discovery"
	"moxa-config/internal/handlers"
	"moxa-config/internal/services"
)

//go:embed web/*
var webFS embed.FS

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.ConfigDir, 0755); err != nil {
		log.Fatalf("Impossible de créer le répertoire de config: %v", err)
	}
	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		log.Fatalf("Impossible de créer le répertoire de backup: %v", err)
	}

	logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Impossible d'ouvrir le fichier de log: %v", err)
	} else {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	// Initialisation du service d'authentification
	authService, err := auth.NewAuthService(filepath.Join(cfg.ConfigDir, "users.json"))
	if err != nil {
		log.Fatalf("Erreur initialisation auth: %v", err)
	}
	authHandler := auth.NewHandler(authService, cfg.TLSEnabled)

	// Initialisation du service de découverte
	discoveryService, err := discovery.NewService()
	if err != nil {
		log.Printf("Avertissement: service de découverte non disponible: %v", err)
	} else {
		if err := discoveryService.Start(); err != nil {
			log.Printf("Avertissement: impossible de démarrer le service de découverte: %v", err)
		} else {
			defer discoveryService.Stop()
		}
	}

	networkService := services.NewNetworkService(cfg.ConfigDir)
	serialService := services.NewSerialService(cfg.ConfigDir)

	networkHandler := handlers.NewNetworkHandler(networkService)
	serialHandler := handlers.NewSerialHandler(serialService)

	mux := http.NewServeMux()

	// Routes d'authentification (non protégées)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/check", authHandler.CheckSession)
	mux.HandleFunc("/api/auth/change-password", authHandler.ChangePassword)

	// Routes API protégées
	apiMux := http.NewServeMux()
	apiMux.Handle("/api/network/", networkHandler)
	apiMux.Handle("/api/serial/", serialHandler)
	apiMux.HandleFunc("/api/system/info", handleSystemInfo)
	apiMux.HandleFunc("/api/health", handleHealth)

	// Appliquer le middleware d'authentification aux routes API
	mux.Handle("/api/network/", authService.Middleware(apiMux))
	mux.Handle("/api/serial/", authService.Middleware(apiMux))
	mux.Handle("/api/system/", authService.Middleware(apiMux))
	mux.Handle("/api/health", authService.Middleware(apiMux))

	// Fichiers statiques
	webContent, _ := fs.Sub(webFS, "web")
	fileServer := http.FileServer(http.FS(webContent))
	mux.Handle("/", fileServer)

	// Logging middleware
	handler := loggingMiddleware(mux)

	// Configuration du serveur
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	protocol := "HTTP"
	if cfg.TLSEnabled {
		protocol = "HTTPS"
		certFile := filepath.Join(cfg.ConfigDir, "server.crt")
		keyFile := filepath.Join(cfg.ConfigDir, "server.key")

		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			log.Println("Génération d'un certificat auto-signé...")
			if err := generateSelfSignedCert(certFile, keyFile); err != nil {
				log.Fatalf("Erreur génération certificat: %v", err)
			}
		}

		server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		log.Printf("Serveur %s démarré sur le port %s", protocol, cfg.Port)
		log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
	} else {
		log.Printf("Serveur %s démarré sur le port %s", protocol, cfg.Port)
		log.Fatal(server.ListenAndServe())
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hostname, _ := os.Hostname()

	productName := "MOXA UC"
	version := ""
	if out, err := exec.Command("mx-ver").Output(); err == nil {
		line := strings.TrimSpace(string(out))
		parts := strings.SplitN(line, " version ", 2)
		if len(parts) >= 1 {
			productName = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			versionParts := strings.Fields(parts[1])
			if len(versionParts) >= 1 {
				version = versionParts[0]
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data": map[string]string{
			"hostname":    hostname,
			"productName": productName,
			"version":     version,
		},
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"status":  "healthy",
	})
}

func generateSelfSignedCert(certFile, keyFile string) error {
	// Essayer avec openssl d'abord
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:4096",
		"-keyout", keyFile, "-out", certFile,
		"-days", "365", "-nodes",
		"-subj", "/CN=moxa-config/O=MOXA/C=FR")

	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: génération avec Go
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"MOXA"},
			CommonName:   "moxa-config",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	privBytes, _ := x509.MarshalECPrivateKey(priv)
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	os.Chmod(keyFile, 0600)

	return nil
}
