package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	// License configuration
	LicenseSecretName      string
	LicenseSecretNamespace string
	LicenseSecretKey       string

	// Node selector for licensed nodes
	NodeLabelKey   string
	NodeLabelValue string

	// Phone home configuration
	LicenseServerURL    string
	PhoneHomeEnabled    bool
	PhoneHomeInterval   time.Duration
	PhoneHomeRetries    int
	PhoneHomeTimeout    time.Duration

	// Validation configuration
	ValidationInterval  time.Duration
	FailOpen            bool  // If true, allow operations when license is invalid (during grace period)
	GracePeriodDays     int   // Grace period from license

	// Server configuration
	HTTPPort            int
	MetricsPort         int
	HealthCheckInterval time.Duration

	// Logging
	LogLevel            string
	LogFormat           string // json or text
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Defaults
		LicenseSecretName:      getEnv("LICENSE_SECRET_NAME", "es-license"),
		LicenseSecretNamespace: getEnv("LICENSE_SECRET_NAMESPACE", "default"),
		LicenseSecretKey:       getEnv("LICENSE_SECRET_KEY", "license.jwt"),

		NodeLabelKey:   getEnv("NODE_LABEL_KEY", "es-products.io/licensed"),
		NodeLabelValue: getEnv("NODE_LABEL_VALUE", "true"),

		LicenseServerURL:    getEnv("LICENSE_SERVER_URL", ""),
		PhoneHomeEnabled:    getEnvBool("PHONE_HOME_ENABLED", true),
		PhoneHomeInterval:   getEnvDuration("PHONE_HOME_INTERVAL", 24*time.Hour),
		PhoneHomeRetries:    getEnvInt("PHONE_HOME_RETRIES", 3),
		PhoneHomeTimeout:    getEnvDuration("PHONE_HOME_TIMEOUT", 30*time.Second),

		ValidationInterval:  getEnvDuration("VALIDATION_INTERVAL", 5*time.Minute),
		FailOpen:            getEnvBool("FAIL_OPEN", true),

		HTTPPort:            getEnvInt("HTTP_PORT", 8080),
		MetricsPort:         getEnvInt("METRICS_PORT", 9090),
		HealthCheckInterval: getEnvDuration("HEALTH_CHECK_INTERVAL", 30*time.Second),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}

	// Validate required fields
	if cfg.LicenseServerURL == "" && cfg.PhoneHomeEnabled {
		return nil, fmt.Errorf("LICENSE_SERVER_URL is required when PHONE_HOME_ENABLED=true")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
