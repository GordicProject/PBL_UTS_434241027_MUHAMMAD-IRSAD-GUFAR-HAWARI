package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"siakad-api/internal/config"
)

// --- request / response shapes ---

// studentResponse is the shape of a single student in list responses.
type studentResponse struct {
	ID            int64   `json:"id"`
	NIM           string  `json:"nim"`
	Nama          string  `json:"nama"`
	Prodi         string  `json:"prodi"`
	Angkatan      int     `json:"angkatan"`
	IPKTerakhir   *float64 `json:"ipk_terakhir"`
	Email         string  `json:"email"`
	YearCreated   string  `json:"tahun_akademik"`
}

// createStudentRequest is the JSON body for POST /students.
type createStudentRequest struct {
	NIM         string   `json:"nim"`
	Nama        string   `json:"nama"`
	Email       string   `json:"email"`
	Prodi       string   `json:"prodi"`
	Angkatan    int      `json:"angkatan"`
	IPKTerakhir *float64 `json:"ipk_terakhir"`
}

// updateStudentRequest is the JSON body for PUT /students/:id.
type updateStudentRequest struct {
	Nama        string   `json:"nama"`
	Prodi       string   `json:"prodi"`
	Angkatan    int      `json:"angkatan"`
	IPKTerakhir *float64 `json:"ipk_terakhir"`
}

// enrolledCourse is one item in the student detail's course list.
type enrolledCourse struct {
	ID          int64  `json:"id"`
	KodeMK      string `json:"kode_mk"`
	NamaMK      string `json:"nama_mk"`
	SKS         int    `json:"sks"`
	TahunAkademik string `json:"tahun_akademik"`
}

// studentDetailResponse is the GET /students/:id response data.
type studentDetailResponse struct {
	ID           int64          `json:"id"`
	NIM          string         `json:"nim"`
	Nama         string         `json:"nama"`
	Prodi        string         `json:"prodi"`
	Angkatan     int           `json:"angkatan"`
	IPKTerakhir  *float64      `json:"ipk_terakhir"`
	Email        string         `json:"email"`
	Courses      []enrolledCourse `json:"courses"`
	TotalSKS     int          `json:"total_sks"`
	BatasSKS     int          `json:"batas_sks"`
}

// --- helpers ---

// sksLimitFromIPK returns the max SKS allowed for an academic year.
func sksLimitFromIPK(ipk *float64) int {
	if ipk == nil || *ipk < 2.50 {
		return 18
	}
	if *ipk >= 3.00 {
		return 24
	}
	return 21
}

// validateAngkatan checks angkatan is 4 digits and <= current year.
func validateAngkatan(angkatan int) error {
	currentYear := time.Now().Year()
	if angkatan < 1000 || angkatan > currentYear {
		return fmt.Errorf("angkatan harus 4 digit dan tidak lebih dari tahun berjalan (%d)", currentYear)
	}
	return nil
}

// validateNIM checks NIM is 12 digits.
func validateNIM(nim string) error {
	if len(nim) != 12 {
		return errors.New("NIM harus 12 digit angka")
	}
	for _, ch := range nim {
		if ch < '0' || ch > '9' {
			return errors.New("NIM harus terdiri dari angka saja")
		}
	}
	return nil
}

// parseStudentID extracts and parses the :id param.
func parseStudentID(c *fiber.Ctx) (int64, error) {
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, errors.New("ID tidak valid")
	}
	return id, nil
}

// --- handlers ---

// ListStudents — GET /students (admin).
// Query params: page, per_page, prodi, angkatan, search, sort.
func ListStudents(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		page := 1
		perPage := 10
		if v := c.Query("page"); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				page = p
			}
		}
		if v := c.Query("per_page"); v != "" {
			if pp, err := strconv.Atoi(v); err == nil && pp > 0 {
				perPage = pp
				if perPage > 50 {
					perPage = 50
				}
			}
		}

		prodi := c.Query("prodi")
		angkatan := c.Query("angkatan")
		search := c.Query("search")
		sort := c.Query("sort")

		// Build dynamic WHERE clauses
		where := []string{"s.deleted_at IS NULL"}
		args := []any{}

		if prodi != "" {
			args = append(args, prodi)
			where = append(where, fmt.Sprintf("s.prodi = $%d", len(args)))
		}
		if angkatan != "" {
			args = append(args, angkatan)
			where = append(where, fmt.Sprintf("s.angkatan = $%d", len(args)))
		}
		if search != "" {
			like := "%" + search + "%"
			args = append(args, like, like)
			where = append(where, fmt.Sprintf("(s.nim LIKE $%d OR s.nama LIKE $%d)", len(args)-1, len(args)))
		}

		// Default sort: by ID
		orderClause := "s.id"
		switch sort {
		case "nama":
			orderClause = "s.nama ASC"
		case "-ipk_terakhir":
			orderClause = "s.ipk_terakhir DESC NULLS LAST"
		case "ipk_terakhir":
			orderClause = "s.ipk_terakhir ASC NULLS LAST"
		}

		// Count total
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM students s WHERE %s", strings.Join(where, " AND "))
		var total int
		countArgs := args // same args, in the same order
		if err := pool.QueryRow(c.Context(), countQuery, countArgs...).Scan(&total); err != nil {
			log.Printf("list students count error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		offset := (page - 1) * perPage

		// Build SELECT with LIMIT/OFFSET appended to the args
		whereClause := strings.Join(where, " AND ")
		selectQuery := fmt.Sprintf(
			"SELECT s.id, s.nim, s.nama, s.prodi, s.angkatan, s.ipk_terakhir, u.email "+
				"FROM students s JOIN users u ON u.id = s.user_id "+
				"WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d",
			whereClause, orderClause, len(args)+1, len(args)+2,
		)
		selectArgs := append(args, perPage, offset)

		rows, err := pool.Query(c.Context(), selectQuery, selectArgs...)
		if err != nil {
			log.Printf("list students query error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		defer rows.Close()

		var students []studentResponse
		for rows.Next() {
			var s studentResponse
			var ipk *float64
			err := rows.Scan(&s.ID, &s.NIM, &s.Nama, &s.Prodi, &s.Angkatan, &ipk, &s.Email)
			if err != nil {
				log.Printf("list students scan error: %v", err)
				return Error(c, fiber.StatusInternalServerError, "Internal server error")
			}
			if ipk != nil {
				s.IPKTerakhir = ipk
			}
			students = append(students, s)
		}
		if students == nil {
			students = []studentResponse{}
		}

		lastPage := (total + perPage - 1) / perPage
		if lastPage < 1 {
			lastPage = 1
		}

		meta := &Meta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    lastPage,
		}

		return Success(c, fiber.StatusOK, "Daftar mahasiswa", students, meta)
	}
}

// CreateStudent — POST /students (admin).
func CreateStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req createStudentRequest
		if ok, _ := bindBody(c, &req); !ok {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"body": {"request body tidak valid"},
			})
		}

		// --- Validate ---
		validationErrs := map[string][]string{}

		if req.NIM == "" {
			validationErrs["nim"] = append(validationErrs["nim"], "NIM wajib diisi")
		} else if err := validateNIM(req.NIM); err != nil {
			validationErrs["nim"] = append(validationErrs["nim"], err.Error())
		}
		if req.Nama == "" {
			validationErrs["nama"] = append(validationErrs["nama"], "nama wajib diisi")
		}
		if req.Email == "" {
			validationErrs["email"] = append(validationErrs["email"], "email wajib diisi")
		}
		if req.Prodi == "" {
			validationErrs["prodi"] = append(validationErrs["prodi"], "prodi wajib diisi")
		}
		if req.Angkatan == 0 {
			validationErrs["angkatan"] = append(validationErrs["angkatan"], "angkatan wajib diisi")
		} else if err := validateAngkatan(req.Angkatan); err != nil {
			validationErrs["angkatan"] = append(validationErrs["angkatan"], err.Error())
		}
		if req.IPKTerakhir != nil {
			if *req.IPKTerakhir < 0 || *req.IPKTerakhir > 4.00 {
				validationErrs["ipk_terakhir"] = append(validationErrs["ipk_terakhir"], "IPK harus 0.00 - 4.00")
			}
		}

		if len(validationErrs) > 0 {
			return ValidationError(c, "Validasi gagal", validationErrs)
		}

		// --- Check uniqueness ---
		var nimCount int
		if err := pool.QueryRow(c.Context(),
			`SELECT COUNT(*) FROM students WHERE nim = $1`, req.NIM,
		).Scan(&nimCount); err != nil {
			log.Printf("nim uniqueness check error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if nimCount > 0 {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"nim": {"NIM sudah terdaftar"},
			})
		}

		var emailCount int
		if err := pool.QueryRow(c.Context(),
			`SELECT COUNT(*) FROM users WHERE email = $1`, req.Email,
		).Scan(&emailCount); err != nil {
			log.Printf("email uniqueness check error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if emailCount > 0 {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"email": {"email sudah terdaftar"},
			})
		}

		// --- Transaction: create user + student ---
		ctx := c.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Printf("begin tx error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		defer tx.Rollback(ctx)

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.NIM), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("bcrypt error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		var userID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO users (email, password, role) VALUES ($1, $2, 'mahasiswa') RETURNING id`,
			req.Email, string(hashed),
		).Scan(&userID)
		if err != nil {
			log.Printf("insert user error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		var ipk *float64
		if req.IPKTerakhir != nil {
			ipk = req.IPKTerakhir
		}

		var studentID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO students (user_id, nim, nama, prodi, angkatan, ipk_terakhir)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			userID, req.NIM, req.Nama, req.Prodi, req.Angkatan, ipk,
		).Scan(&studentID)
		if err != nil {
			log.Printf("insert student error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		if err := tx.Commit(ctx); err != nil {
			log.Printf("commit tx error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		created := studentResponse{
			ID:          studentID,
			NIM:         req.NIM,
			Nama:        req.Nama,
			Prodi:       req.Prodi,
			Angkatan:    req.Angkatan,
			IPKTerakhir: req.IPKTerakhir,
			Email:       req.Email,
		}
		return Success(c, fiber.StatusCreated, "Mahasiswa berhasil dibuat", created, nil)
	}
}

// GetStudent — GET /students/:id (admin, or mahasiswa for own data).
func GetStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := parseStudentID(c)
		if err != nil {
			return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
		}

		role, _ := c.Locals("role").(string)
		userID, _ := c.Locals("user_id").(int64)

		// For mahasiswa, ensure they are only accessing their own record
		if role == "mahasiswa" {
			// Check if the student's user_id matches the caller's user_id
			var ownUserID int64
			if err := pool.QueryRow(c.Context(),
				`SELECT user_id FROM students WHERE id = $1 AND deleted_at IS NULL`, id,
			).Scan(&ownUserID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
				}
				log.Printf("get student db error: %v", err)
				return Error(c, fiber.StatusInternalServerError, "Internal server error")
			}
			if ownUserID != userID {
				return Error(c, fiber.StatusForbidden, "Anda hanya dapat mengakses data Anda sendiri")
			}
		}

		// Fetch student data
		var s studentDetailResponse
		var ipk *float64
		var email string
		err = pool.QueryRow(c.Context(),
			`SELECT s.id, s.nim, s.nama, s.prodi, s.angkatan, s.ipk_terakhir, u.email
			 FROM students s JOIN users u ON u.id = s.user_id
			 WHERE s.id = $1 AND s.deleted_at IS NULL`,
			id,
		).Scan(&s.ID, &s.NIM, &s.Nama, &s.Prodi, &s.Angkatan, &ipk, &email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
			}
			log.Printf("get student db error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		s.Email = email
		if ipk != nil {
			s.IPKTerakhir = ipk
		}

		// Get enrolled courses for the active academic year
		ctx := c.Context()
		courses, totalSKS, err := fetchStudentCourses(pool, ctx, s.ID, cfg.TahunAkademikAktif)
		if err != nil {
			log.Printf("fetch courses error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		s.Courses = courses
		s.TotalSKS = totalSKS
		s.BatasSKS = sksLimitFromIPK(s.IPKTerakhir)

		return Success(c, fiber.StatusOK, "Data mahasiswa", s, nil)
	}
}

// fetchStudentCourses retrieves enrolled courses and total SKS for a student
// in the given academic year.
func fetchStudentCourses(pool *pgxpool.Pool, ctx context.Context, studentID int64, tahunAkademik string) ([]enrolledCourse, int, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.course_id, c.kode_mk, c.nama_mk, c.sks, e.tahun_akademik
		 FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE e.student_id = $1 AND e.tahun_akademik = $2
		 ORDER BY c.kode_mk`,
		studentID, tahunAkademik,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var courses []enrolledCourse
	totalSKS := 0
	for rows.Next() {
		var ec enrolledCourse
		if err := rows.Scan(&ec.ID, &ec.KodeMK, &ec.NamaMK, &ec.SKS, &ec.TahunAkademik); err != nil {
			return nil, 0, err
		}
		courses = append(courses, ec)
		totalSKS += ec.SKS
	}
	if courses == nil {
		courses = []enrolledCourse{}
	}
	return courses, totalSKS, nil
}

// UpdateStudent — PUT /students/:id (admin).
func UpdateStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := parseStudentID(c)
		if err != nil {
			return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
		}

		var req updateStudentRequest
		if ok, _ := bindBody(c, &req); !ok {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"body": {"request body tidak valid"},
			})
		}

		// --- Validate (same rules as create) ---
		validationErrs := map[string][]string{}

		if req.Nama == "" {
			validationErrs["nama"] = append(validationErrs["nama"], "nama wajib diisi")
		}
		if req.Prodi == "" {
			validationErrs["prodi"] = append(validationErrs["prodi"], "prodi wajib diisi")
		}
		if req.Angkatan == 0 {
			validationErrs["angkatan"] = append(validationErrs["angkatan"], "angkatan wajib diisi")
		} else if err := validateAngkatan(req.Angkatan); err != nil {
			validationErrs["angkatan"] = append(validationErrs["angkatan"], err.Error())
		}
		if req.IPKTerakhir != nil {
			if *req.IPKTerakhir < 0 || *req.IPKTerakhir > 4.00 {
				validationErrs["ipk_terakhir"] = append(validationErrs["ipk_terakhir"], "IPK harus 0.00 - 4.00")
			}
		}

		if len(validationErrs) > 0 {
			return ValidationError(c, "Validasi gagal", validationErrs)
		}

		// Verify student exists and is not soft-deleted
		var exists bool
		if err := pool.QueryRow(c.Context(),
			`SELECT true FROM students WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
			}
			log.Printf("update student check error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		var ipk *float64
		if req.IPKTerakhir != nil {
			ipk = req.IPKTerakhir
		}

		tag, err := pool.Exec(c.Context(),
			`UPDATE students SET nama = $2, prodi = $3, angkatan = $4, ipk_terakhir = $5
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, req.Nama, req.Prodi, req.Angkatan, ipk,
		)
		if err != nil {
			log.Printf("update student exec error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if tag.RowsAffected() == 0 {
			return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
		}

		// Fetch updated record
		var s studentResponse
		var ipkVal *float64
		err = pool.QueryRow(c.Context(),
			`SELECT s.id, s.nim, s.nama, s.prodi, s.angkatan, s.ipk_terakhir, u.email
			 FROM students s JOIN users u ON u.id = s.user_id
			 WHERE s.id = $1`,
			id,
		).Scan(&s.ID, &s.NIM, &s.Nama, &s.Prodi, &s.Angkatan, &ipkVal, &s.Email)
		if err != nil {
			log.Printf("update student re-fetch error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if ipkVal != nil {
			s.IPKTerakhir = ipkVal
		}

		return Success(c, fiber.StatusOK, "Data mahasiswa diperbarui", s, nil)
	}
}

// DeleteStudent — DELETE /students/:id (admin, soft delete).
func DeleteStudent(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := parseStudentID(c)
		if err != nil {
			return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
		}

		var exists bool
		if err := pool.QueryRow(c.Context(),
			`SELECT true FROM students WHERE id = $1 AND deleted_at IS NULL`, id,
		).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
			}
			log.Printf("delete student check error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		tag, err := pool.Exec(c.Context(),
			`UPDATE students SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
			id,
		)
		if err != nil {
			log.Printf("delete student exec error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if tag.RowsAffected() == 0 {
			return Error(c, fiber.StatusNotFound, "Student tidak ditemukan")
		}

		return SuccessNoContent(c)
	}
}

