package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	SourceDir        string
	TargetDir        string
	DeleteStagingDir string
	ConfigDir        string
	Port             string
	TZ               string
	MaxConcurrency   int
}

// Load reads configuration from environment variables with sane defaults.
func Load() *Config {
	maxConcurrency := 4
	if v := os.Getenv("MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrency = n
		}
	}

	return &Config{
		SourceDir:        getEnv("SOURCE_DIR", "/data/source"),
		TargetDir:        getEnv("TARGET_DIR", "/data/target"),
		DeleteStagingDir: getEnv("DELETE_STAGING_DIR", "/data/delete_staging"),
		ConfigDir:        getEnv("CONFIG_DIR", "/data/config"),
		Port:             getEnv("PORT", "8080"),
		TZ:               getEnv("TZ", "Asia/Shanghai"),
		MaxConcurrency:   maxConcurrency,
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultVal
}
