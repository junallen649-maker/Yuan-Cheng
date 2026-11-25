package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DBPath             string
	JWTSecret          string
	AdminDefaultUser   string
	AdminDefaultPass   string
	OfflineSeconds     int
	ServerPublicHost   string
	RendezvousPort     int
	RelayPort          int
}

func Load() *Config {
	offlineSec := 45
	if v := os.Getenv("POLARIS_OFFLINE_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 15 {
			offlineSec = parsed
		}
	}
	return &Config{
		Port:             getEnv("POLARIS_PORT", "21114"),
		DBPath:           getEnv("POLARIS_DB_PATH", "./data/polaris.db"),
		JWTSecret:        os.Getenv("POLARIS_JWT_SECRET"),
		AdminDefaultUser: getEnv("POLARIS_ADMIN_USER", "admin"),
		AdminDefaultPass: getEnv("POLARIS_ADMIN_PASS", "polaris2026"),
		OfflineSeconds:   offlineSec,
		ServerPublicHost: getEnv("POLARIS_PUBLIC_HOST", "8.138.129.79"),
		RendezvousPort:   21116,
		RelayPort:        21117,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
