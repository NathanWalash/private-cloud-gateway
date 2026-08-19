package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func doSetup(h *Handler, body map[string]string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	h.Setup(rec, httptest.NewRequest("POST", "/api/auth/setup", bytes.NewReader(b)))
	return rec
}

func validSetup(token string) map[string]string {
	m := map[string]string{"email": "admin@example.com", "password": "password123", "first_name": "A"}
	if token != "" {
		m["token"] = token
	}
	return m
}

func TestSetupTokenRequiredRejectsWrongOrMissingToken(t *testing.T) {
	for _, tok := range []string{"", "wrong"} {
		h := &Handler{db: setupTestDB(t), setupToken: "s3cret"}
		rec := doSetup(h, validSetup(tok))
		if rec.Code != 403 {
			t.Errorf("token %q: want 403, got %d", tok, rec.Code)
		}
	}
}

func TestSetupTokenRequiredAcceptsCorrectToken(t *testing.T) {
	h := &Handler{db: setupTestDB(t), setupToken: "s3cret"}
	rec := doSetup(h, validSetup("s3cret"))
	if rec.Code != 201 {
		t.Fatalf("want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestSetupWithoutConfiguredTokenAllows(t *testing.T) {
	h := &Handler{db: setupTestDB(t)} // no setupToken (dev)
	rec := doSetup(h, validSetup(""))
	if rec.Code != 201 {
		t.Fatalf("want 201, got %d (%s)", rec.Code, rec.Body.String())
	}
}
