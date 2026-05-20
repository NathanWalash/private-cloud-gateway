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
	first_name TEXT     NOT NULL DEFAULT '',
	last_name  TEXT     NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS apps (
	id              INTEGER  PRIMARY KEY AUTOINCREMENT,
	blueprint_id    TEXT     NOT NULL UNIQUE,
	name            TEXT     NOT NULL,
	icon            TEXT     NOT NULL DEFAULT '📦',
	subdomain       TEXT     NOT NULL UNIQUE,
	internal_port   INTEGER  NOT NULL,
	image           TEXT     NOT NULL,
	container_name  TEXT     NOT NULL UNIQUE,
	status          TEXT     NOT NULL DEFAULT 'stopped',
	created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_apps_status ON apps(status);

CREATE TABLE IF NOT EXISTS settings (
	key        TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS monitors (
	id           INTEGER  PRIMARY KEY AUTOINCREMENT,
	name         TEXT     NOT NULL,
	url          TEXT     NOT NULL UNIQUE,
	status       TEXT     NOT NULL DEFAULT 'unknown',
	status_code  INTEGER,
	latency_ms   INTEGER,
	last_checked DATETIME,
	created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// Additive migrations — safe to run on existing DBs.
	// SQLite returns "duplicate column name" if the column exists; we ignore that.
	for _, col := range []string{
		"ALTER TABLE users ADD COLUMN first_name TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN last_name  TEXT NOT NULL DEFAULT ''",
	} {
		_, _ = db.Exec(col) // ignore error — column may already exist
	}
	return nil
}
