package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// ListCourses — GET /courses (all authenticated roles). Step 1 stub.
func ListCourses(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNotImplemented) }
}
