package database

import "database/sql"

func RunMigrations(db *sql.DB) error {
	queries := []string{
		`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL
		);
		`,
		`
		CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx ON users (email);
		`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	return nil
}
