package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// Login is the POST /auth/login handler (public, rate-limited).
// Step 1 stub — full implementation in step 2.
func Login(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotImplemented)
	}
}

// Me is the GET /auth/me handler (all roles).
// Step 1 stub — full implementation in step 2.
func Me(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotImplemented)
	}
}
