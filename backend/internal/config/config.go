package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv       string
	AppPort      string
	AppURL       string
	AppSecret    string
	DatabaseURL  string
	SupabaseURL  string
	SupabaseKey  string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
	SessionTTL   time.Duration
	CSRFSecret   string
	ArgonMemory  uint32
	ArgonIter    uint32
	ArgonParallel uint8
	IsProduction bool
}

func Load() (*Config, error) {
	_ = godotenvLoad()

	sessionHours := getEnvInt("SESSION_DURATION_HOURS", 168)
	// Vercel / container platforms inject PORT; fall back to APP_PORT for local Docker.
	port := os.Getenv("PORT")
	if port == "" {
		port = getEnv("APP_PORT", "8080")
	}
	cfg := &Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		AppPort:       port,
		AppURL:        getEnv("APP_URL", "http://localhost:8080"),
		AppSecret:     getEnv("APP_SECRET", ""),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		SupabaseURL:   getEnv("SUPABASE_URL", ""),
		SupabaseKey:   getEnv("SUPABASE_SERVICE_KEY", ""),
		SMTPHost:      getEnv("SMTP_HOST", ""),
		SMTPPort:      getEnvInt("SMTP_PORT", 587),
		SMTPUser:      getEnv("SMTP_USER", ""),
		SMTPPass:      getEnv("SMTP_PASS", ""),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@findneighbour.app"),
		SessionTTL:    time.Duration(sessionHours) * time.Hour,
		CSRFSecret:    getEnv("CSRF_SECRET", ""),
		ArgonMemory:   uint32(getEnvInt("ARGON2_MEMORY", 65536)),
		ArgonIter:     uint32(getEnvInt("ARGON2_ITERATIONS", 3)),
		ArgonParallel: uint8(getEnvInt("ARGON2_PARALLELISM", 4)),
	}
	cfg.IsProduction = cfg.AppEnv == "production"

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AppSecret == "" {
		return nil, fmt.Errorf("APP_SECRET is required")
	}
	if len(cfg.AppSecret) < 32 {
		return nil, fmt.Errorf("APP_SECRET must be at least 32 characters")
	}
	if cfg.CSRFSecret == "" {
		cfg.CSRFSecret = cfg.AppSecret
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func godotenvLoad() error {
	if _, err := os.Stat(".env"); err != nil {
		return nil
	}
	return loadDotEnv()
}
