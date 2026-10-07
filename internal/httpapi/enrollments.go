package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// CreateEnrollment — POST /enrollments (mahasiswa only). Step 1 stub.
func CreateEnrollment(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}

// DeleteEnrollment — DELETE /enrollments/:id (mahasiswa, own record). Step 1 stub.
func DeleteEnrollment(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}
