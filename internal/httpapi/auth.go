package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/mail"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"siakad-api/internal/config"
)

// loginRequest is the JSON body for POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// userResponse is the user object returned in login and me responses.
type userResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// studentInfo is the students data included in /auth/me for mahasiswa role.
type studentInfo struct {
	NIM      string `json:"nim"`
	Nama     string `json:"nama"`
	Prodi    string `json:"prodi"`
	Angkatan int    `json:"angkatan"`
}

// loginResponse is the full successful login response.
type loginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        userResponse `json:"user"`
}

// meResponse is the full successful me response.
type meResponse struct {
	User     userResponse `json:"user"`
	Students *studentInfo `json:"students,omitempty"`
}

// Login is the POST /auth/login handler (public, rate-limited by middleware).
func Login(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()

		// --- Bind body ---
		var req loginRequest
		if ok, _ := bindBody(c, &req); !ok {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"body": {"request body tidak valid"},
			})
		}

		// --- Validate ---
		validationErrs := map[string][]string{}
		emailOK := false
		if req.Email == "" {
			validationErrs["email"] = append(validationErrs["email"], "email wajib diisi")
		} else if _, err := mail.ParseAddress(req.Email); err != nil {
			validationErrs["email"] = append(validationErrs["email"], "format email tidak valid")
		} else {
			emailOK = true
		}
		if len(req.Password) < 8 {
			validationErrs["password"] = append(validationErrs["password"], "password minimal 8 karakter")
		}
		if emailOK && len(validationErrs) == 0 {
			// All good, continue
		} else if len(validationErrs) > 0 {
			// 422 validation errors should NOT count toward the rate limiter
			return ValidationError(c, "Validasi gagal", validationErrs)
		}

		// --- Look up user by email ---
		var user struct {
			ID       int64
			Email    string
			Password string
			Role     string
		}
		err := pool.QueryRow(c.Context(),
			`SELECT id, email, password, role FROM users WHERE email = $1`,
			req.Email,
		).Scan(&user.ID, &user.Email, &user.Password, &user.Role)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				RecordLoginFailure(ip)
				return Error(c, fiber.StatusUnauthorized, "Kredensial salah")
			}
			log.Printf("login db error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		// --- Check if mahasiswa and soft-deleted ---
		if user.Role == "mahasiswa" {
			var isDeleted bool
			if err := pool.QueryRow(c.Context(),
				`SELECT deleted_at IS NOT NULL FROM students WHERE user_id = $1`,
				user.ID,
			).Scan(&isDeleted); err == nil && isDeleted {
				RecordLoginFailure(ip)
				return Error(c, fiber.StatusUnauthorized, "Kredensial salah")
			}
		}

		// --- Verify password ---
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			RecordLoginFailure(ip)
			return Error(c, fiber.StatusUnauthorized, "Kredensial salah")
		}

		// --- Success: clear rate-limit counter, issue token ---
		ClearLoginAttempts(ip)

		expiresIn := int(cfg.JWTExpires.Seconds())
		claims := &Claims{
			UserID: user.ID,
			Email:  user.Email,
			Role:   user.Role,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.JWTExpires)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				Issuer:    "siakad-api",
			},
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			log.Printf("jwt sign error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		return Success(c, fiber.StatusOK, "Login berhasil", loginResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   expiresIn,
			User: userResponse{
				ID:    user.ID,
				Email: user.Email,
				Role:  user.Role,
			},
		}, nil)
	}
}

// Me is the GET /auth/me handler (all roles, must be called after JWTMiddleware).
func Me(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(int64)
		email := c.Locals("email").(string)
		role := c.Locals("role").(string)

		resp := meResponse{
			User: userResponse{ID: userID, Email: email, Role: role},
		}

		// If mahasiswa, attach students data
		if role == "mahasiswa" {
			var si studentInfo
			err := pool.QueryRow(c.Context(),
				`SELECT nim, nama, prodi, angkatan
				 FROM students WHERE user_id = $1 AND deleted_at IS NULL`,
				userID,
			).Scan(&si.NIM, &si.Nama, &si.Prodi, &si.Angkatan)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					// Soft-deleted: omit students sub-object, return 200
					return Success(c, fiber.StatusOK, fmt.Sprintf("Data user %s", email), resp, nil)
				}
				log.Printf("me db error: %v", err)
				return Error(c, fiber.StatusInternalServerError, "Internal server error")
			}
			resp.Students = &si
		}

		return Success(c, fiber.StatusOK, fmt.Sprintf("Data user %s", email), resp, nil)
	}
}
