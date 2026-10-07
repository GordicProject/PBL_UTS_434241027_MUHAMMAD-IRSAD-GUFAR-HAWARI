package httpapi

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Claims are the JWT claims stored in every token.
type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTMiddleware verifies the Bearer token and loads user claims into the
// Fiber context under the keys "user_id", "email", and "role".
// It must run before role-gated middleware.
func JWTMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if auth == "" {
			return Error(c, fiber.StatusUnauthorized, "Authorization token is required")
		}
		token, err := extractToken(auth)
		if err != nil {
			return Error(c, fiber.StatusUnauthorized, "Invalid or expired token")
		}

		claims := &Claims{}
		_, err = jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			return Error(c, fiber.StatusUnauthorized, "Invalid or expired token")
		}

		// Set on the context for downstream handlers
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)
		c.Locals("jwt_expires_at", time.Now().Add(24*time.Hour).Unix())
		return c.Next()
	}
}

// RequireAdmin is a fiber middleware that must run after JWTMiddleware.
// It rejects any request whose role is not "admin".
func RequireAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "admin" {
		log.Printf("403 admin denied: user=%v role=%s", c.Locals("user_id"), role)
		return Error(c, fiber.StatusForbidden, "Admin role required")
	}
	return c.Next()
}

// RequireMahasiswa is a fiber middleware that must run after JWTMiddleware.
// It rejects any request whose role is not "mahasiswa".
func RequireMahasiswa(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "mahasiswa" {
		log.Printf("403 mahasiswa denied: user=%v role=%s", c.Locals("user_id"), role)
		return Error(c, fiber.StatusForbidden, "Mahasiswa role required")
	}
	return c.Next()
}

// RequireMahasiswaOrAdmin allows both mahasiswa and admin roles through.
func RequireMahasiswaOrAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "mahasiswa" && role != "admin" {
		return Error(c, fiber.StatusForbidden, "Mahasiswa or admin role required")
	}
	return c.Next()
}

// extractToken pulls the token string from an Authorization header value.
func extractToken(auth string) (string, error) {
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}
	return parts[1], nil
}
