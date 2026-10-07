CREATE TABLE students (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL UNIQUE REFERENCES users(id),
    nim          VARCHAR(12) NOT NULL UNIQUE,
    nama         VARCHAR(255) NOT NULL,
    prodi        VARCHAR(100) NOT NULL,
    angkatan     INT NOT NULL,
    ipk_terakhir NUMERIC(3,2) NULL,
    deleted_at   TIMESTAMP NULL,
    CONSTRAINT chk_students_nim       CHECK (nim ~ '^[0-9]{12}$'),
    CONSTRAINT chk_students_angkatan  CHECK (angkatan BETWEEN 1000 AND 9999),
    CONSTRAINT chk_students_ipk       CHECK (ipk_terakhir BETWEEN 0 AND 4)
);

CREATE INDEX idx_students_angkatan ON students(angkatan);
