package httpapi

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-api/internal/config"
)

// courseResponse is the shape of a single course in the list response.
type courseResponse struct {
	ID        int64  `json:"id"`
	KodeMK    string `json:"kode_mk"`
	NamaMK    string `json:"nama_mk"`
	SKS       int    `json:"sks"`
	Kuota     int    `json:"kuota"`
	Semester  int    `json:"semester"`
	Terisi    int    `json:"terisi"`
	SisaKuota int    `json:"sisa_kuota"`
}

// ListCourses — GET /courses (all authenticated roles).
// Query params: semester, search (kode_mk or nama_mk), available=true (only not-full).
func ListCourses(pool *pgxpool.Pool, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		semester := c.Query("semester")
		search := c.Query("search")
		availableOnly := c.Query("available") == "true"

		where := []string{}
		args := []any{}

		if semester != "" {
			args = append(args, semester)
			where = append(where, fmt.Sprintf("c.semester = $%d", len(args)))
		}
		if search != "" {
			like := "%" + search + "%"
			args = append(args, like, like)
			where = append(where, fmt.Sprintf("(c.kode_mk LIKE $%d OR c.nama_mk LIKE $%d)", len(args)-1, len(args)))
		}

		// Build the full query:
		//   WHERE args (semester, search) → $1..$N
		//   TA arg (tahun_akademik)      → $(N+1)
		whereClause := ""
		if len(where) > 0 {
			whereClause = "WHERE " + strings.Join(where, " AND ")
		}

		taParamNum := len(args) + 1
		args = append(args, cfg.TahunAkademikAktif)

		groupHaving := "GROUP BY c.id, c.kode_mk, c.nama_mk, c.sks, c.quota, c.semester"
		if availableOnly {
			groupHaving += " HAVING COUNT(e.id) < c.quota"
		}

		query := fmt.Sprintf(
			"SELECT c.id, c.kode_mk, c.nama_mk, c.sks, c.quota, c.semester, "+
				"COUNT(e.id) AS terisi, "+
				"GREATEST(c.quota - COUNT(e.id), 0) AS sisa_kuota "+
				"FROM courses c "+
				"LEFT JOIN enrollments e ON e.course_id = c.id AND e.tahun_akademik = $%d "+
				"%s %s ORDER BY c.kode_mk ASC",
			taParamNum, whereClause, groupHaving,
		)

		rows, err := pool.Query(c.Context(), query, args...)
		if err != nil {
			log.Printf("list courses query error: %v", err)
			return Error(c, fiber.StatusInternalServerError, "Internal server error")
		}
		defer rows.Close()

		var courses []courseResponse
		for rows.Next() {
			var cr courseResponse
			if err := rows.Scan(&cr.ID, &cr.KodeMK, &cr.NamaMK, &cr.SKS, &cr.Kuota, &cr.Semester, &cr.Terisi, &cr.SisaKuota); err != nil {
				log.Printf("list courses scan error: %v", err)
				return Error(c, fiber.StatusInternalServerError, "Internal server error")
			}
			courses = append(courses, cr)
		}
		if courses == nil {
			courses = []courseResponse{}
		}

		return Success(c, fiber.StatusOK, "Daftar mata kuliah", courses, nil)
	}
}
