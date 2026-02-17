// Package config gère la configuration de l'application.
package config

import "os"

// Config contient la configuration de l'application.
type Config struct {
	Port       string
	ConfigDir  string
	BackupDir  string
	LogFile    string
	TLSEnabled bool
}

// Load charge la configuration depuis les variables d'environnement.
func Load() *Config {
	return &Config{
		Port:       getEnv("MOXA_PORT", "8080"),
		ConfigDir:  getEnv("MOXA_CONFIGDIR", "/etc/moxa-config"),
		BackupDir:  getEnv("MOXA_BACKUPDIR", "/var/moxa-config/backups"),
		LogFile:    getEnv("MOXA_LOGFILE", "/var/log/moxa-config.log"),
		TLSEnabled: getEnv("MOXA_TLS", "true") == "true",
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
