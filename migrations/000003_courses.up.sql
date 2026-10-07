CREATE TABLE courses (
    id        BIGSERIAL PRIMARY KEY,
    kode_mk   VARCHAR(20) NOT NULL UNIQUE,
    nama_mk   VARCHAR(255) NOT NULL,
    sks       INT NOT NULL,
    quota     INT NOT NULL,
    semester  INT NOT NULL,
    CONSTRAINT chk_courses_sks      CHECK (sks BETWEEN 2 AND 4),
    CONSTRAINT chk_courses_quota    CHECK (quota > 0),
    CONSTRAINT chk_courses_semester CHECK (semester BETWEEN 1 AND 8)
);
