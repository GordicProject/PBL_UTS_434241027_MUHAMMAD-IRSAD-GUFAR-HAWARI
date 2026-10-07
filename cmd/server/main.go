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

	// --- Public: login with rate limiting ---
	v1.Post("/auth/login", httpapi.RateLimitLogin(), httpapi.Login(pool, cfg))

	// --- Authenticated (any role) ---
	auth := v1.Group("")
	auth.Use(httpapi.JWTMiddleware(cfg.JWTSecret))
	auth.Get("/auth/me", httpapi.Me(pool, cfg))

	// --- Admin only ---
	admin := v1.Group("")
	admin.Use(httpapi.JWTMiddleware(cfg.JWTSecret), httpapi.RequireAdmin)
	admin.Get("/students", httpapi.ListStudents(pool, cfg))
	admin.Post("/students", httpapi.CreateStudent(pool, cfg))
	admin.Put("/students/:id", httpapi.UpdateStudent(pool, cfg))
	admin.Delete("/students/:id", httpapi.DeleteStudent(pool, cfg))

	// --- Admin + mahasiswa self-access on GET /students/:id ---
	// The combined group runs JWTMiddleware; the handler itself checks
	// whether the caller is admin or the owning mahasiswa.
	combined := v1.Group("")
	combined.Use(httpapi.JWTMiddleware(cfg.JWTSecret))
	combined.Get("/students/:id", httpapi.GetStudent(pool, cfg))

	// --- All authenticated roles ---
	roles := v1.Group("")
	roles.Use(httpapi.JWTMiddleware(cfg.JWTSecret))
	roles.Get("/courses", httpapi.ListCourses(pool, cfg))

	// --- Mahasiswa only ---
	mhs := v1.Group("")
	mhs.Use(httpapi.JWTMiddleware(cfg.JWTSecret), httpapi.RequireMahasiswa)
	mhs.Post("/enrollments", httpapi.CreateEnrollment(pool, cfg))
	mhs.Delete("/enrollments/:id", httpapi.DeleteEnrollment(pool, cfg))

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
