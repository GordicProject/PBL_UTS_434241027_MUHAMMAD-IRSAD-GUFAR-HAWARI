package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	AdminEmail  string
	AdminPassword string
	TahunAkademikAktif string
	Port              string
	JWTExpires        time.Duration
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}
	taAktif := os.Getenv("TAHUN_AKADEMIK_AKTIF")
	if taAktif == "" {
		taAktif = "2026/2027-Ganjil"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return &Config{
		DatabaseURL:        dbURL,
		JWTSecret:          secret,
		AdminEmail:         "admin@siakad.test",
		AdminPassword:      adminPassword,
		TahunAkademikAktif: taAktif,
		Port:               port,
		JWTExpires:         24 * time.Hour,
	}, nil
}
