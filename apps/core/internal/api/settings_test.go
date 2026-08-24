package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetSettings_EmptyReturnsEmptyArray(t *testing.T) {
	h := &Handler{db: newTestDB(t)}
	rec := httptest.NewRecorder()
	h.GetSettings(rec, httptest.NewRequest("GET", "/api/settings", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
}

func TestPutSetting_RejectsMissingKey(t *testing.T) {
	h := &Handler{db: newTestDB(t)}
	for _, path := range []string{"/api/settings/", "/api/settings"} {
		rec := httptest.NewRecorder()
		h.PutSetting(rec, httptest.NewRequest("PUT", path, strings.NewReader(`{"value":"x"}`)))
		if rec.Code != 400 {
			t.Errorf("PutSetting %q = %d, want 400", path, rec.Code)
		}
	}
}

func TestPutSetting_RejectsInvalidBody(t *testing.T) {
	h := &Handler{db: newTestDB(t)}
	rec := httptest.NewRecorder()
	h.PutSetting(rec, httptest.NewRequest("PUT", "/api/settings/THEME", strings.NewReader("not json")))
	if rec.Code != 400 {
		t.Errorf("PutSetting with invalid JSON = %d, want 400", rec.Code)
	}
}

func TestPutSetting_PersistsAndReadsBack(t *testing.T) {
	db := newTestDB(t)
	h := &Handler{db: db}

	rec := httptest.NewRecorder()
	h.PutSetting(rec, httptest.NewRequest("PUT", "/api/settings/THEME", strings.NewReader(`{"value":"dark"}`)))
	if rec.Code != 200 {
		t.Fatalf("PutSetting = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.GetSettings(rec, httptest.NewRequest("GET", "/api/settings", nil))
	var settings []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings) != 1 || settings[0].Key != "THEME" || settings[0].Value != "dark" {
		t.Errorf("settings = %+v, want one THEME=dark row", settings)
	}
}
