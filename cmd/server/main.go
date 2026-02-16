// Package main est le point d'entrée de moxa-config.
// Ce serveur fournit une interface web et API REST pour configurer
// les interfaces réseau et ports série des équipements MOXA.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"moxa-config/internal/config"
	"moxa-config/internal/handlers"
	"moxa-config/internal/services"
)

//go:embed web/*
var webFS embed.FS

func main() {
	cfg := config.Load()

	if err := ensureDirs(cfg); err != nil {
		log.Fatalf("Erreur création répertoires: %v", err)
	}

	setupLogging(cfg.LogFile)

	networkSvc := services.NewNetworkService(cfg.BackupDir)
	serialSvc := services.NewSerialService(cfg.ConfigDir)

	networkHandler := handlers.NewNetworkHandler(networkSvc)
	serialHandler := handlers.NewSerialHandler(serialSvc)

	mux := http.NewServeMux()

	mux.Handle("/api/network/", networkHandler)
	mux.Handle("/api/network", networkHandler)
	mux.Handle("/api/serial/", serialHandler)
	mux.Handle("/api/serial", serialHandler)

	mux.HandleFunc("/api/health", healthCheck)
	mux.HandleFunc("/api/system/info", systemInfo)

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Erreur chargement web: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      logMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Démarrage serveur sur le port %d", cfg.Port)
	log.Printf("Interface web: http://localhost:%d", cfg.Port)
	log.Printf("API réseau:    http://localhost:%d/api/network", cfg.Port)
	log.Printf("API série:     http://localhost:%d/api/serial", cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Erreur serveur: %v", err)
	}
}

func ensureDirs(cfg *config.Config) error {
	dirs := []string{cfg.ConfigDir, cfg.BackupDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("impossible de créer %s: %w", dir, err)
		}
	}
	return nil
}

func setupLogging(logFile string) {
	if logFile == "" {
		return
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Impossible d'ouvrir le fichier log %s: %v", logFile, err)
		return
	}

	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func healthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func systemInfo(w http.ResponseWriter, _ *http.Request) {
	hostname, _ := os.Hostname()

	info := map[string]string{
		"hostname": hostname,
		"version":  "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true,"data":{"hostname":"%s","version":"%s"}}`,
		info["hostname"], info["version"])
}
