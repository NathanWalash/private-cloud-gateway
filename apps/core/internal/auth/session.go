package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const sessionTTL = 8 * time.Hour

func newSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func createSession(db *sql.DB, userID int64) (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	_, err = db.Exec(
		"INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)",
		id, userID, time.Now().Add(sessionTTL),
	)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return id, nil
}

// validateSession returns the user ID for a valid, non-expired session.
// Returns 0, nil if the session does not exist or is expired.
func validateSession(db *sql.DB, sessionID string) (int64, error) {
	var userID int64
	err := db.QueryRow(
		"SELECT user_id FROM sessions WHERE id = ? AND expires_at > CURRENT_TIMESTAMP",
		sessionID,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("validate session: %w", err)
	}
	return userID, nil
}

func deleteSession(db *sql.DB, sessionID string) error {
	if _, err := db.Exec("DELETE FROM sessions WHERE id = ?", sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func auditLog(db *sql.DB, action, actor, detail string) {
	// Best-effort — never block the request path on audit failures.
	_, _ = db.Exec(
		"INSERT INTO audit_log (action, actor, detail) VALUES (?, ?, ?)",
		action, actor, detail,
	)
}
