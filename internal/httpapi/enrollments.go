package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// validTARegex matches the tahun_akademik format: 2026/2027-Ganjil or 2026/2027-Genap
var validTARegex = regexp.MustCompile(`^\d{4}/\d{4}-(Ganjil|Genap)$`)

// createEnrollmentRequest is the JSON body for POST /enrollments.
type createEnrollmentRequest struct {
	CourseID       int64  `json:"course_id"`
	TahunAkademik  string `json:"tahun_akademik"`
}

// --- handlers ---

// CreateEnrollment — POST /enrollments (mahasiswa only, via RequireMahasiswa middleware).
func CreateEnrollment(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// The caller is a mahasiswa (enforced by RequireMahasiswa)
		userID := c.Locals("user_id").(int64)

		// --- Bind body ---
		var req createEnrollmentRequest
		if ok, _ := bindBody(c, &req); !ok {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"body": {"request body tidak valid"},
			})
		}

		// --- Validate ---
		validationErrs := map[string][]string{}
		if req.CourseID <= 0 {
			validationErrs["course_id"] = append(validationErrs["course_id"], "course_id wajib dan harus valid")
		}
		if req.TahunAkademik == "" {
			validationErrs["tahun_akademik"] = append(validationErrs["tahun_akademik"], "tahun_akademik wajib diisi")
		} else if !validTARegex.MatchString(req.TahunAkademik) {
			validationErrs["tahun_akademik"] = append(validationErrs["tahun_akademik"],
				"format tahun akademik tidak valid, contoh: 2026/2027-Ganjil")
		}
		if len(validationErrs) > 0 {
			return ValidationError(c, "Validasi gagal", validationErrs)
		}

		// --- Get the student record for this user ---
		var studentID int64
		var ipk *float64
		if err := pool.QueryRow(c.Context(),
			`SELECT id, ipk_terakhir FROM students WHERE user_id = $1 AND deleted_at IS NULL`,
			userID,
		).Scan(&studentID, &ipk); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Data mahasiswa tidak ditemukan")
			}
			log.Printf("enrollment student lookup error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		// --- Transaction with row locking ---
		ctx := c.Context()
		tx, err := pool.Begin(ctx)
		defer tx.Rollback(ctx)
		if err != nil {
			log.Printf("enrollment begin tx error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		defer tx.Rollback(ctx)

		// 1. Lock the course row (prevents concurrent quota changes)
		var course struct {
			ID      int64
			KodeMK  string
			NamaMK  string
			SKS     int
			Kuota   int
		}
		err = tx.QueryRow(ctx,
			`SELECT id, kode_mk, nama_mk, sks, quota FROM courses WHERE id = $1 FOR UPDATE`,
			req.CourseID,
		).Scan(&course.ID, &course.KodeMK, &course.NamaMK, &course.SKS, &course.Kuota)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Mata kuliah tidak ditemukan")
			}
			log.Printf("enrollment course lock error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		// 2. Check duplicate enrollment (same student, same course, same TA)
		var dupCount int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM enrollments
			 WHERE student_id = $1 AND course_id = $2 AND tahun_akademik = $3`,
			studentID, req.CourseID, req.TahunAkademik,
		).Scan(&dupCount); err != nil {
			log.Printf("enrollment dup check error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if dupCount > 0 {
			return Error(c, fiber.StatusConflict, "Anda sudah mengambil mata kuliah ini pada tahun akademik tersebut")
		}

		// 3. Check quota (count current enrollments for this course + TA)
		var enrolledCount int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND tahun_akademik = $2`,
			req.CourseID, req.TahunAkademik,
		).Scan(&enrolledCount); err != nil {
			log.Printf("enrollment quota count error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if enrolledCount >= course.Kuota {
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"course_id": {"kuota mata kuliah sudah penuh"},
			})
		}

		// 4. Check SKS limit
		currentTotalSKS, err := fetchTotalSKSForStudent(tx, ctx, studentID, req.TahunAkademik)
		if err != nil {
			log.Printf("enrollment SKS sum error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		limit := sksLimitFromIPK(ipk)
		remaining := limit - currentTotalSKS
		if remaining < course.SKS {
			msg := fmt.Sprintf("Total SKS (%d) + mata kuliah ini (%d SKS) melebihi batas %d SKS. Sisa SKS: %d",
				currentTotalSKS, course.SKS, limit, remaining)
			return ValidationError(c, "Validasi gagal", map[string][]string{
				"course_id": {msg},
			})
		}

		// 5. Insert the enrollment
		var enrollmentID int64
		err = tx.QueryRow(ctx,
			`INSERT INTO enrollments (student_id, course_id, tahun_akademik)
			 VALUES ($1, $2, $3) RETURNING id`,
			studentID, req.CourseID, req.TahunAkademik,
		).Scan(&enrollmentID)
		if err != nil {
			log.Printf("enrollment insert error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		if err := tx.Commit(ctx); err != nil {
			log.Printf("enrollment commit error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		resp := map[string]any{
			"id":              enrollmentID,
			"course_id":       req.CourseID,
			"course":          map[string]any{"kode_mk": course.KodeMK, "nama_mk": course.NamaMK, "sks": course.SKS},
			"tahun_akademik":  req.TahunAkademik,
			"total_sks_sekarang": currentTotalSKS + course.SKS,
			"batas_sks":       limit,
		}
		return Success(c, fiber.StatusCreated, "Mata kuliah berhasil diambil", resp, nil)
	}
}

// fetchTotalSKSForStudent returns the sum of SKS for a student in a given TA.
func fetchTotalSKSForStudent(tx pgx.Tx, ctx context.Context, studentID int64, tahunAkademik string) (int, error) {
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(c.sks), 0)
		 FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE e.student_id = $1 AND e.tahun_akademik = $2`,
		studentID, tahunAkademik,
	).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// DeleteEnrollment — DELETE /enrollments/:id (mahasiswa, own record only).
func DeleteEnrollment(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		idStr := c.Params("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return Error(c, fiber.StatusNotFound, "Enrollment tidak ditemukan")
		}

		userID := c.Locals("user_id").(int64)

		// Get the student record
		var studentID int64
		if err := pool.QueryRow(c.Context(),
			`SELECT id FROM students WHERE user_id = $1 AND deleted_at IS NULL`,
			userID,
		).Scan(&studentID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Data mahasiswa tidak ditemukan")
			}
			log.Printf("delete enrollment student lookup error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		// Check the enrollment exists and belongs to this student
		var ownerStudentID int64
		if err := pool.QueryRow(c.Context(),
			`SELECT student_id FROM enrollments WHERE id = $1`,
			id,
		).Scan(&ownerStudentID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Error(c, fiber.StatusNotFound, "Enrollment tidak ditemukan")
			}
			log.Printf("delete enrollment lookup error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		if ownerStudentID != studentID {
			return Error(c, fiber.StatusForbidden, "Anda hanya dapat menghapus enrollment milik Anda sendiri")
		}

		if _, err := pool.Exec(c.Context(),
			`DELETE FROM enrollments WHERE id = $1 AND student_id = $2`,
			id, studentID,
		); err != nil {
			log.Printf("delete enrollment exec error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}

		return SuccessNoContent(c)
	}
}
