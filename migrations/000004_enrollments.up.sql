CREATE TABLE enrollments (
    id             BIGSERIAL PRIMARY KEY,
    student_id     BIGINT NOT NULL REFERENCES students(id),
    course_id      BIGINT NOT NULL REFERENCES courses(id),
    tahun_akademik VARCHAR(20) NOT NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_enrollments UNIQUE (student_id, course_id, tahun_akademik),
    CONSTRAINT chk_enrollments_ta CHECK (tahun_akademik ~ '^[0-9]{4}/[0-9]{4}-(Ganjil|Genap)$')
);

CREATE INDEX idx_enrollments_student_id ON enrollments(student_id);
CREATE INDEX idx_enrollments_course_id  ON enrollments(course_id);
