package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"

	"siakad-api/internal/config"
	"siakad-api/internal/db"
	"siakad-api/internal/httpapi"
)

func main() {
	if err := loadEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	app := fiber.New(fiber.Config{
		ErrorHandler: httpapi.ErrorHandler,
	})

	// Health check (no auth)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")

	// --- Public (no JWT) ---
	v1.Post("/auth/login", httpapi.RateLimitLogin(), httpapi.Login(pool, cfg))

	// All remaining routes require a valid JWT. Register JWT as a single
	// global middleware on v1 so it runs before every route handler below.
	// Using app.Use() instead of Group("") avoids Fiber v2's empty-prefix
	// route-tree pollution where multiple Group("") instances share the
	// same tree node and leak middlewares into each other's routes.
	app.Use(httpapi.JWTMiddleware(cfg.JWTSecret))

	// --- Authenticated, any role ---
	v1.Get("/auth/me", httpapi.Me(pool, cfg))

	// --- Admin-only (role checked inline in handler wrapper) ---
	withAdmin := func(h fiber.Handler) fiber.Handler {
		return httpapi.RequireAdmin(h)
	}
	v1.Get("/students", withAdmin(httpapi.ListStudents(pool, cfg)))
	v1.Post("/students", withAdmin(httpapi.CreateStudent(pool, cfg)))
	v1.Put("/students/:id", withAdmin(httpapi.UpdateStudent(pool, cfg)))
	v1.Delete("/students/:id", withAdmin(httpapi.DeleteStudent(pool, cfg)))

	// GET /students/:id — admin or owning mahasiswa
	v1.Get("/students/:id", httpapi.GetStudent(pool, cfg))

	// --- All authenticated roles ---
	v1.Get("/courses", httpapi.ListCourses(pool, cfg))

	// --- Mahasiswa-only (role checked inline in handler wrapper) ---
	withMhs := func(h fiber.Handler) fiber.Handler {
		return httpapi.RequireMahasiswa(h)
	}
	v1.Post("/enrollments", withMhs(httpapi.CreateEnrollment(pool, cfg)))
	v1.Delete("/enrollments/:id", withMhs(httpapi.DeleteEnrollment(pool, cfg)))

	addr := ":" + cfg.Port
	log.Printf("SIAKAD Mini server starting on %s", addr)

	// Graceful shutdown on SIGINT / SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
		if err := app.Shutdown(); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	if err := app.Listen(addr); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// loadEnv reads a .env file and sets environment variables that are not
// already set. Mirrors the pattern in cmd/migrate/main.go.
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
