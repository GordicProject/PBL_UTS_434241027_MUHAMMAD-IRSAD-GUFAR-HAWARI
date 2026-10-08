package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"siakad-api/internal/config"
	"siakad-api/internal/db"
)

func main() {
	if err := loadEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	fmt.Println("Seeding database...")

	// --- 1. Admin user ---
	adminEmail := cfg.AdminEmail
	if adminEmail == "" {
		adminEmail = "admin@siakad.test"
	}
	adminHash, _ := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)

	var adminCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE email = $1`, adminEmail,
	).Scan(&adminCount); err != nil {
		log.Fatalf("admin check: %v", err)
	}
	if adminCount == 0 {
		_, err = pool.Exec(ctx,
			`INSERT INTO users (email, password, role)
			 VALUES ($1, $2, 'admin')
			 ON CONFLICT (email) DO NOTHING`,
			adminEmail, string(adminHash),
		)
		if err != nil {
			log.Fatalf("admin insert: %v", err)
		}
		fmt.Printf("Created admin: %s\n", adminEmail)
	} else {
		fmt.Println("Admin already exists, skipping.")
	}

	// --- 2. 20 mahasiswa ---
	// Variasi IPK agar tiap batas SKS bisa diuji:
	//   5 mahasiswa ipk >= 3.00 (batas 24)
	//   5 mahasiswa ipk 2.50-2.99 (batas 21)
	//   5 mahasiswa ipk < 2.50 (batas 18)
	//   5 mahasiswa ipk NULL (batas 18)
	studentNames := []string{
		"Ahmad Fauzi", "Siti Rahayu", "Budi Santoso", "Dewi Lestari", "Eko Prasetyo",
		"Fitri Handayani", "Galang Saputra", "Hana Wijaya", "Iqbal Ramadhan", "Joko Susilo",
		"Kirana Ayu", "Lukman Hakim", "Maya Sari", "Nanda Pratama", "Oka Permana",
		"Putri Ananda", "Rizky Maulana", "Sari Indah", "Taufik Hidayat", "Wulan Maharani",
	}
	prodis := []string{"Teknik Informatika", "Sistem Informasi", "Ilmu Komputer"}
	angkatanList := []int{2022, 2023, 2024, 2025}
	// IPK variations to cover all limits
	// 0-4:  >= 3.00 (max 24)
	// 5-9:  2.50-2.99 (max 21)
	// 10-14: < 2.50 (max 18)
	// 15-19: NULL (max 18)
	ipkValues := []float64{
		3.00, 3.25, 3.50, 3.75, 4.00, // >= 3.00
		2.50, 2.60, 2.70, 2.80, 2.99,  // 2.50-2.99
		1.80, 2.00, 2.20, 2.40, 1.50,  // < 2.50
		0, 0, 0, 0, 0,                 // NULL (ipk = 0 means we set nil)
	}
	isNullIPK := []bool{
		false, false, false, false, false,
		false, false, false, false, false,
		false, false, false, false, false,
		true, true, true, true, true,
	}

	for i, name := range studentNames {
		// NIM: 12 digits, e.g. 202210000001, 202210000002, ...
		nim := fmt.Sprintf("%012d", 202210000001+i)
		email := fmt.Sprintf("mahasiswa%d@siakad.test", i+1)
		prodi := prodis[i%3]
		angkatan := angkatanList[i%4]

		var ipk *float64
		if !isNullIPK[i] {
			v := ipkValues[i]
			ipk = &v
		}

		// Check if student already exists by NIM
		var exists int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM students WHERE nim = $1`, nim,
		).Scan(&exists); err != nil {
			log.Fatalf("check student %s: %v", nim, err)
		}
		if exists > 0 {
			continue
		}

		// Hash NIM as the initial password
		hashed, _ := bcrypt.GenerateFromPassword([]byte(nim), bcrypt.DefaultCost)

		// Insert user: check-then-insert (deterministic, no ON CONFLICT RETURNING)
		var userExists int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM users WHERE email = $1`, email,
		).Scan(&userExists); err != nil {
			log.Fatalf("check user %s: %v", email, err)
		}
		var userID int64
		if userExists > 0 {
			if err := pool.QueryRow(ctx,
				`SELECT id FROM users WHERE email = $1`, email,
			).Scan(&userID); err != nil {
				log.Fatalf("lookup user %s: %v", email, err)
			}
		} else {
			if err := pool.QueryRow(ctx,
				`INSERT INTO users (email, password, role)
				 VALUES ($1, $2, 'mahasiswa') RETURNING id`,
				email, string(hashed),
			).Scan(&userID); err != nil {
				log.Fatalf("insert user %s: %v", email, err)
			}
		}

		// Insert student: idempotent — check by NIM before inserting.
		// The outer NIM check (line ~113) already skips if the student exists;
		// this second check guards against a race between the outer check and the
		// insert (e.g. two parallel seeder runs).
		var stuCount int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM students WHERE nim = $1`, nim,
		).Scan(&stuCount); err != nil {
			log.Fatalf("check student %s: %v", nim, err)
		}
		if stuCount == 0 {
			_, err := pool.Exec(ctx,
				`INSERT INTO students (user_id, nim, nama, prodi, angkatan, ipk_terakhir)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				userID, nim, name, prodi, angkatan, ipk,
			)
			if err != nil {
				log.Fatalf("insert student %s: %v", nim, err)
			}
		}
	}
	fmt.Println("Seeded 20 mahasiswa.")

	// --- 3. 10 mata kuliah ---
	// Variasi SKS (2-4) dan kuota (salah satu kuota kecil untuk uji kuota penuh)
	type courseSeed struct {
		kode, nama string
		sks, quota, semester int
	}
	courses := []courseSeed{
		{"IF101", "Algoritma dan Pemrograman", 4, 50, 1},
		{"IF102", "Matematika Diskrit", 3, 40, 1},
		{"IF201", "Struktur Data", 4, 35, 2},
		{"IF202", "Basis Data", 3, 30, 2},
		{"IF301", "Jaringan Komputer", 3, 25, 3},
		{"IF302", "Sistem Operasi", 4, 20, 3},
		{"IF401", "Kecerdasan Buatan", 3, 15, 4},
		{"IF402", "Rekayasa Perangkat Lunak", 3, 10, 4},
		{"IF501", "Sistem Terdistribusi", 4, 5, 5},  // kuota kecil untuk uji kuota penuh
		{"IF502", "Pendidikan Teknologi Informasi", 2, 50, 5},
	}

	for _, cs := range courses {
		var exists int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM courses WHERE kode_mk = $1`, cs.kode,
		).Scan(&exists); err != nil {
			log.Fatalf("check course %s: %v", cs.kode, err)
		}
		if exists > 0 {
			continue
		}

		_, err := pool.Exec(ctx,
			`INSERT INTO courses (kode_mk, nama_mk, sks, quota, semester)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (kode_mk) DO NOTHING`,
			cs.kode, cs.nama, cs.sks, cs.quota, cs.semester,
		)
		if err != nil {
			log.Fatalf("insert course %s: %v", cs.kode, err)
		}
	}
	fmt.Println("Seeded 10 mata kuliah.")

	fmt.Println("Seeding complete.")
}

// loadEnv reads a .env file and sets environment variables that are not
// already set.
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

