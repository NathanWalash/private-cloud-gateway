package db

import (
	"database/sql"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// NeedsSetup returns true if no users exist yet (first run).
func NeedsSetup(db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// CreateUser creates a new user with bcrypt-hashed password.
func CreateUser(db *sql.DB, email, password, firstName, lastName string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.Exec(
		"INSERT INTO users (email, password, first_name, last_name) VALUES (?, ?, ?, ?)",
		email, string(hash), firstName, lastName,
	)
	return err
}

// Bootstrap creates the first admin user if no users exist.
// Used by CLOUD_CORE_BOOTSTRAP_* env vars for headless first-run.
// Deprecated in favour of the in-app setup wizard — kept for backward compatibility.
func Bootstrap(db *sql.DB, email, password string) error {
	needs, err := NeedsSetup(db)
	if err != nil {
		return fmt.Errorf("check existing users: %w", err)
	}
	if !needs {
		return fmt.Errorf("users already exist, skipping bootstrap")
	}
	return CreateUser(db, email, password, "", "")
}
