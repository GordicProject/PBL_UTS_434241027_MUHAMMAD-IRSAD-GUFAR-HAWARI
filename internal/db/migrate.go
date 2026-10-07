package db

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies all pending up migrations using the file:// driver.
func RunMigrations(dsn string) error {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("migrate: already up to date")
			return nil
		}
		return fmt.Errorf("migrate.Up: %w", err)
	}
	log.Println("migrate: all migrations applied")
	return nil
}
