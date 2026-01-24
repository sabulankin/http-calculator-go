-- +goose Up
CREATE TABLE IF NOT EXISTS calculations (
                                            id SERIAL PRIMARY KEY,
                                            expression TEXT NOT NULL,
                                            result DOUBLE PRECISION NOT NULL,
                                            created_at TIMESTAMP NOT NULL DEFAULT now()
    );

-- +goose Down
DROP TABLE IF EXISTS calculations;
