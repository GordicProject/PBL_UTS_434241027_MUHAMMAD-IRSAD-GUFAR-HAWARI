package main

import (
	"log"
	"os"
	"strings"

	"siakad-api/internal/config"
	"siakad-api/internal/db"
)

func main() {
	if err := loadEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	_ = cfg
	dsn := os.Getenv("DATABASE_URL")

	dir := "up"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	switch dir {
	case "up":
		if err := db.RunMigrations(dsn); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
	case "down":
		if err := db.DownMigrations(dsn); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
	default:
		log.Fatalf("unknown direction: %s (use up or down)", dir)
	}
}

func loadEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return nil
}
