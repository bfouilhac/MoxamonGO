// Package config gère la configuration de l'application.
package config

import (
	"os"
	"strconv"
)

// Config contient les paramètres de configuration du serveur.
type Config struct {
	Port       int
	WebRoot    string
	ConfigDir  string
	BackupDir  string
	LogFile    string
	AllowedIPs []string
}

// Load charge la configuration depuis les variables d'environnement ou les valeurs par défaut.
func Load() *Config {
	return &Config{
		Port:      getEnvInt("MOXA_PORT", 8080),
		WebRoot:   getEnv("MOXA_WEBROOT", "/opt/moxa-config/web"),
		ConfigDir: getEnv("MOXA_CONFIGDIR", "/etc/moxa-config"),
		BackupDir: getEnv("MOXA_BACKUPDIR", "/var/moxa-config/backups"),
		LogFile:   getEnv("MOXA_LOGFILE", "/var/log/moxa-config.log"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
