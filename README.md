# SIAKAD Mini (KRS) — UTS Pemrograman Backend Lanjut

API server untuk KRS mini dengan 10 endpoint, JWT authentication, role-based access,
dan kuota/SKS enforcement.

## Teknologi

- **Go 1.22+** (versi 1.27 di lingkungan ini)
- **Fiber v2** — HTTP framework
- **golang-jwt/jwt/v5** — JWT HS256
- **pgxpool** (jackc/pgx/v5) — PostgreSQL connection pool
- **golang.org/x/crypto/bcrypt** — password hashing
- **golang-migrate** — database migrations

## Struktur Project

```
.
├── cmd/
│   ├── migrate/main.go       # Jalankan migrasi DB (up/down)
│   ├── seed/main.go          # Seeder data awal (idempoten)
│   └── server/main.go        # Entry point API server
├── internal/
│   ├── config/config.go      # Config dari env
│   ├── db/
│   │   ├── db.go             # pgxpool.Open
│   │   └── migrate.go        # RunMigrations / DownMigrations
│   └── httpapi/
│       ├── responses.go      # Helper respons JSON seragam
│       ├── middleware.go     # JWT auth + role middleware
│       ├── ratelimit.go      # Rate limiter login per IP
│       ├── auth.go           # POST /auth/login, GET /auth/me
│       ├── students.go       # GET/POST/PUT/DELETE /students
│       ├── courses.go        # GET /courses
│       └── enrollments.go    # POST/DELETE /enrollments
├── migrations/               # SQL migrations (JANGAN diubah)
├── .env.example              # Template konfigurasi
└── go.mod
```

## Setup

### 1. Siapkan PostgreSQL

Pastikan PostgreSQL berjalan dan database `siakad` sudah dibuat:

```bash
createdb siakad
```

### 2. Salin dan edit `.env`

```bash
cp .env.example .env
```

Edit `.env` sesuai kebutuhan. Variabel yang dibaca:

| Variabel | Deskripsi | Default |
|----------|-----------|---------|
| `DATABASE_URL` | DSN PostgreSQL | *(wajib)* |
| `JWT_SECRET` | Kunci JWT HS256 | *(wajib)* |
| `JWT_EXPIRES_HOURS` | Masa berlaku token (jam) | `24` |
| `ADMIN_EMAIL` | Email admin | `admin@siakad.test` |
| `ADMIN_PASSWORD` | Password admin | *(wajib)* |
| `TAHUN_AKADEMIK_AKTIF` | TA untuk hitung SKS | `2026/2027-Ganjil` |
| `PORT` | Port server | `8080` |

### 3. Jalankan Migrasi

```bash
# Tambahkan Go ke PATH (Windows)
$env:Path = "C:\Program Files\Go\bin;" + $env:Path

go run ./cmd/migrate up
```

### 4. Jalankan Seeder

Seeder idempoten — aman dijalankan berulang kali. Membuat:
- 1 admin (password dari `ADMIN_PASSWORD`)
- 20 mahasiswa (NIM 12 digit, password awal = NIM di-hash bcrypt)
- 10 mata kuliah (variasi SKS dan kuota, termasuk 1 kuota kecil untuk uji kuota penuh)

```bash
go run ./cmd/seed
```

### 5. Jalankan Server

```bash
go run ./cmd/server
```

Server mendengarkan di `:8080` (atau `PORT` dari `.env`).

## Endpoint

Semua endpoint diawali `/api/v1`. Semua endpoint **wajib** `Authorization: Bearer <token>` kecuali `POST /auth/login`.

| Method | Endpoint | Role | Deskripsi |
|--------|----------|------|-----------|
| POST | `/auth/login` | Publik | Login, dapatkan JWT token |
| GET | `/auth/me` | Semua | Data user + mahasiswa (jika role mahasiswa) |
| GET | `/students` | Admin | Daftar mahasiswa (paginasi, filter, sort) |
| POST | `/students` | Admin | Buat mahasiswa baru |
| GET | `/students/{id}` | Admin / Mahasiswa (milik sendiri) | Detail mahasiswa + KRS + total SKS |
| PUT | `/students/{id}` | Admin | Ubah data mahasiswa |
| DELETE | `/students/{id}` | Admin | Soft delete mahasiswa |
| GET | `/courses` | Semua | Daftar mata kuliah + terisi/sisa kuota |
| POST | `/enrollments` | Mahasiswa | Ambil mata kuliah (cek kuota + SKS) |
| DELETE | `/enrollments/{id}` | Mahasiswa | Batalkan pengambiln mata kuliah |

## Business Rules

- **Batas SKS** (per tahun akademik, berdasarkan `ipk_terakhir`):
  - IPK >= 3.00 → maks 24 SKS
  - IPK 2.50–2.99 → maks 21 SKS
  - IPK < 2.50 atau NULL → maks 18 SKS
- Tidak boleh mengambil mata kuliah yang sama dua kali pada tahun akademik yang sama.
- Mata kuliah dengan kuota penuh tidak bisa diambil.
- Mahasiswa hanya bisa mengakses dan mengubah KRS miliknya sendiri.

## Format Respons

**Sukses:**
```json
{
  "success": true,
  "message": "...",
  "data": ...,
  "meta": { "current_page": 1, "per_page": 10, "total": 20, "last_page": 2 }
}
```
`meta` hanya ada untuk list.

**Error:**
```json
{ "success": false, "message": "..." }
```

**Validasi (422):**
```json
{
  "success": false,
  "message": "Validasi gagal",
  "errors": { "nim": ["NIM sudah terdaftar"] }
}
```

## Status Code

`200` OK · `201` Created · `204` No Content · `401` Unauthorized ·
`403` Forbidden · `404` Not Found · `409` Conflict ·
`422` Unprocessable Entity · `429` Too Many Requests · `500` Internal Server Error

## Catatan

- Email admin default: `admin@siakad.test` (bisa override lewat `ADMIN_EMAIL` di `.env`)
- Mahasiswa yang sudah di-soft-delete tidak bisa login (401)
- Rate limit login: >5 kegagalan per IP per menit → 429
