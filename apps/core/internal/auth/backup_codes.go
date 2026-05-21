package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	numBackupCodes    = 8
	backupCodeDigits  = 10
)

// GenerateBackupCodes creates numBackupCodes random codes, stores bcrypt hashes,
// and returns the plaintext codes (shown to user once).
func GenerateBackupCodes(db *sql.DB, userID int64) ([]string, error) {
	// Remove existing codes
	_, _ = db.Exec("DELETE FROM totp_backup_codes WHERE user_id=?", userID)

	codes := make([]string, numBackupCodes)
	for i := range codes {
		n, err := rand.Int(rand.Reader, big.NewInt(1e10))
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
		plain := fmt.Sprintf("%010d", n.Int64())
		hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
		if err != nil {
			return nil, err
		}
		_, err = db.Exec("INSERT INTO totp_backup_codes(user_id, code_hash) VALUES(?,?)", userID, string(hash))
		if err != nil {
			return nil, err
		}
		codes[i] = plain
	}
	return codes, nil
}

// UseBackupCode attempts to consume a backup code. Returns true if valid and unused.
func UseBackupCode(db *sql.DB, userID int64, code string) bool {
	rows, err := db.Query(
		"SELECT id, code_hash FROM totp_backup_codes WHERE user_id=? AND used_at IS NULL", userID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var hash string
		if rows.Scan(&id, &hash) != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			// Mark as used
			db.Exec("UPDATE totp_backup_codes SET used_at=? WHERE id=?", time.Now(), id) //nolint:errcheck
			return true
		}
	}
	return false
}

// BackupCodeStatus returns how many backup codes remain unused.
func BackupCodeStatus(db *sql.DB, userID int64) (total, unused int) {
	db.QueryRow("SELECT COUNT(*) FROM totp_backup_codes WHERE user_id=?", userID).Scan(&total) //nolint:errcheck
	db.QueryRow("SELECT COUNT(*) FROM totp_backup_codes WHERE user_id=? AND used_at IS NULL", userID).Scan(&unused) //nolint:errcheck
	return
}

// TOTPGenBackupCodes generates backup codes for the authenticated user.
// POST /api/auth/totp/backup-codes
func (h *Handler) TOTPGenBackupCodes(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	codes, err := GenerateBackupCodes(h.db, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]any{"codes": codes, "warning": "Save these now. They will not be shown again."})
	_, _ = w.Write(b)
}

// TOTPBackupCodeStatus returns how many backup codes remain.
// GET /api/auth/totp/backup-codes
func (h *Handler) TOTPBackupCodeStatus(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(cookieName)
	if cookie == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, _ := validateSession(h.db, cookie.Value)
	if userID == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	total, unused := BackupCodeStatus(h.db, userID)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"total":%d,"unused":%d}`, total, unused)
}
