package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL      string
	JWTSecret        string
	AdminEmail       string
	AdminPassword    string
	TahunAkademikAktif string
	Port             string
	JWTExpires       time.Duration
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

	// JWT_EXPIRES_HOURS — default 24h if unset or invalid
	jwtHours := 24
	if v := os.Getenv("JWT_EXPIRES_HOURS"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			jwtHours = h
		}
	}

	taAktif := os.Getenv("TAHUN_AKADEMIK_AKTIF")
	if taAktif == "" {
		taAktif = "2026/2027-Ganjil"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@siakad.test"
	}

	return &Config{
		DatabaseURL:        dbURL,
		JWTSecret:          secret,
		AdminEmail:         adminEmail,
		AdminPassword:      adminPassword,
		TahunAkademikAktif: taAktif,
		Port:               port,
		JWTExpires:         time.Duration(jwtHours) * time.Hour,
	}, nil
}
