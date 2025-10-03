package config

import "os"

type Config struct {
	AuthUsername string
	AuthPassword string
	JWTSecret    string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	WGConfDir    string
	WGClientsDir string
	WGExternalIF string
	WGInterface  string
	WGMode       string
	UIAddr       string
	UIPort       string
}

const (
	DefaultAuthUsername = "ka9"
	DefaultAuthPassword = "333kaa9"
	DefaultJWTSecret    = "fly-me-to-the-moon"
	DefaultDBHost       = "localhost"
	DefaultDBPort       = "5432"
	DefaultDBUser       = "postgres"
	DefaultDBPassword   = "postgres"
	DefaultDBName       = "amneziawg"
	DefaultDBSSLMode    = "disable"
	// Default directory for WireGuard configuration files (writable in development environment). In production, it can be changed to /etc/amnezia/amneziawg via environment variables
	DefaultWGConfDir    = "/etc/amnezia/amneziawg"
	DefaultWGClientsDir = "/etc/amnezia/amneziawg/clients"
	DefaultWGExternalIF = "" // Auto-detect if not set
	DefaultWGInterface  = "awg0"
	DefaultWGMode       = "awg"
	// Frontend listening address/port (service binding). UI_ADDR takes precedence, then UI_PORT
	DefaultUIAddr = "localhost"
	DefaultUIPort = "60000"
)

func LoadConfig() *Config {
	return &Config{
		AuthUsername: getEnvOrDefault("AUTH_USERNAME", DefaultAuthUsername),
		AuthPassword: getEnvOrDefault("AUTH_PASSWORD", DefaultAuthPassword),
		JWTSecret:    getEnvOrDefault("JWT_SECRET", DefaultJWTSecret),
		DBHost:       getEnvOrDefault("DB_HOST", DefaultDBHost),
		DBPort:       getEnvOrDefault("DB_PORT", DefaultDBPort),
		DBUser:       getEnvOrDefault("DB_USER", DefaultDBUser),
		DBPassword:   getEnvOrDefault("DB_PASSWORD", DefaultDBPassword),
		DBName:       getEnvOrDefault("DB_NAME", DefaultDBName),
		DBSSLMode:    getEnvOrDefault("DB_SSL_MODE", DefaultDBSSLMode),
		WGConfDir:    getEnvOrDefault("WG_CONF_DIR", DefaultWGConfDir),
		WGClientsDir: getEnvOrDefault("WG_CLIENTS_DIR", DefaultWGClientsDir),
		WGExternalIF: getEnvOrDefault("WG_EXTERNAL_IF", DefaultWGExternalIF),
		WGInterface:  getEnvOrDefault("WG_INTERFACE", DefaultWGInterface),
		WGMode:       getEnvOrDefault("WG_MODE", DefaultWGMode),
		UIAddr:       getEnvOrDefault("UI_ADDR", DefaultUIAddr),
		UIPort:       getEnvOrDefault("UI_PORT", DefaultUIPort),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
