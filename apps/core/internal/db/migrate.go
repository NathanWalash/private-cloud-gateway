package db

import (
	"database/sql"
	"fmt"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         INTEGER  PRIMARY KEY AUTOINCREMENT,
	email      TEXT     NOT NULL UNIQUE COLLATE NOCASE,
	password   TEXT     NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT     PRIMARY KEY,
	user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS audit_log (
	id         INTEGER  PRIMARY KEY AUTOINCREMENT,
	action     TEXT     NOT NULL,
	actor      TEXT,
	detail     TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
