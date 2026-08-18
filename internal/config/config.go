package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress        string
	DatabaseURL        string
	LogLevel           string
	ShutdownTimeout    time.Duration
	WorkerInterval     time.Duration
	BusinessTimezone   string
	AllowedOrigins     []string
	RateLimitCapacity  int
	RateLimitPerSecond float64
	SessionTTL         time.Duration
	BootstrapUsername  string
	BootstrapPassword  string
	BootstrapName      string
}

func Load() (Config, error) {
	c := Config{
		HTTPAddress:       get("SANITATION_HTTP_ADDRESS", ":8653"),
		DatabaseURL:       get("SANITATION_DATABASE_URL", "file:sanitation.db"),
		LogLevel:          get("SANITATION_LOG_LEVEL", "INFO"),
		BusinessTimezone:  get("SANITATION_BUSINESS_TIMEZONE", "Asia/Shanghai"),
		AllowedOrigins:    splitCSV(get("SANITATION_ALLOWED_ORIGINS", "http://localhost:5173")),
		BootstrapUsername: get("SANITATION_BOOTSTRAP_ADMIN_USERNAME", "admin"),
		BootstrapPassword: os.Getenv("SANITATION_BOOTSTRAP_ADMIN_PASSWORD"),
		BootstrapName:     get("SANITATION_BOOTSTRAP_ADMIN_NAME", "Operations Administrator"),
	}
	var err error
	c.ShutdownTimeout, err = duration("SANITATION_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	c.WorkerInterval, err = duration("SANITATION_WORKER_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	c.SessionTTL, err = duration("SANITATION_SESSION_TTL", 12*time.Hour)
	if err != nil {
		return Config{}, err
	}
	if c.HTTPAddress == "" || c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("http address and database url are required")
	}
	if _, err := time.LoadLocation(c.BusinessTimezone); err != nil {
		return Config{}, fmt.Errorf("business timezone: %w", err)
	}
	c.RateLimitCapacity, err = positiveInt("SANITATION_RATE_LIMIT_CAPACITY", 120)
	if err != nil {
		return Config{}, err
	}
	c.RateLimitPerSecond, err = positiveFloat("SANITATION_RATE_LIMIT_PER_SECOND", 2)
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s must be positive seconds", key)
	}
	return time.Duration(seconds) * time.Second, nil
}

func positiveInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}
func positiveFloat(key string, fallback float64) (float64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive number", key)
	}
	return parsed, nil
}
func splitCSV(value string) []string {
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
