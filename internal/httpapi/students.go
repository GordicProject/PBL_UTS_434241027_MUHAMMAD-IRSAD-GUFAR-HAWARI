package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// ListStudents — GET /students (admin). Step 1 stub.
func ListStudents(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}

// CreateStudent — POST /students (admin). Step 1 stub.
func CreateStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}

// GetStudent — GET /students/:id (admin or owning mahasiswa). Step 1 stub.
func GetStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}

// UpdateStudent — PUT /students/:id (admin). Step 1 stub.
func UpdateStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}

// DeleteStudent — DELETE /students/:id (admin, soft delete). Step 1 stub.
func DeleteStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}
