package db

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Bootstrap creates the first admin user if no users exist.
// It is a no-op once the database has any user record.
func Bootstrap(db *sql.DB, email, password string) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("check existing users: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("users already exist, skipping bootstrap")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	if _, err := db.Exec(
		"INSERT INTO users (email, password) VALUES (?, ?)",
		email, string(hash),
	); err != nil {
		return fmt.Errorf("create bootstrap user: %w", err)
	}

	return nil
}
