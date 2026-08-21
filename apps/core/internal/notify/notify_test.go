package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/db"
)

func TestHTMLEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a & b", "a &amp; b"},
		{"<>&", "&lt;&gt;&amp;"},
	}
	for _, c := range cases {
		got := htmlEscape(c.in)
		if got != c.want {
			t.Errorf("htmlEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitComma(t *testing.T) {
	got := splitComma("a,b,c")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("splitComma: unexpected result %v", got)
	}
}

func TestTrimSp(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  hello  ", "hello"},
		{"no-spaces", "no-spaces"},
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := trimSp(c.in); got != c.want {
			t.Errorf("trimSp(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildMIME(t *testing.T) {
	msg := buildMIME("from@example.com", "to@example.com", "Subject", "Body")
	for _, want := range []string{
		"From: from@example.com",
		"To: to@example.com",
		"Subject: Subject",
		"Body",
		"MIME-Version: 1.0",
	} {
		if !contains(msg, want) {
			t.Errorf("MIME message missing %q\n\nGot:\n%s", want, msg)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRec(s, sub))
}

func containsRec(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Config, event gating, and channel delivery ──────────────────────────────

func testDB(t *testing.T) *sql.DB {
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

func setSetting(t *testing.T, database *sql.DB, key, value string) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO settings(key,value,updated_at) VALUES(?,?,CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEventEnabled(t *testing.T) {
	database := testDB(t)
	s := New(database)

	// Default (no setting) enables everything.
	if !s.eventEnabled(EventBackupDone) {
		t.Error("no NOTIFY_EVENTS setting should enable all events")
	}

	setSetting(t, database, "NOTIFY_EVENTS", "all")
	if !s.eventEnabled(EventLoginFail) {
		t.Error(`"all" should enable every event`)
	}

	setSetting(t, database, "NOTIFY_EVENTS", "none")
	if s.eventEnabled(EventBackupDone) {
		t.Error(`"none" should disable every event`)
	}

	setSetting(t, database, "NOTIFY_EVENTS", "monitor.down, backup.failed")
	if !s.eventEnabled(EventMonitorDown) {
		t.Error("an event in the list should be enabled")
	}
	if !s.eventEnabled(EventBackupFailed) {
		t.Error("a whitespace-padded event in the list should be enabled")
	}
	if s.eventEnabled(EventLoginSuccess) {
		t.Error("an event not in the list should be disabled")
	}
}

func TestConfigReaders(t *testing.T) {
	database := testDB(t)
	s := New(database)

	if tok, chat := s.telegramConfig(); tok != "" || chat != "" {
		t.Error("telegramConfig should be empty when unset")
	}
	setSetting(t, database, "TELEGRAM_BOT_TOKEN", "tok")
	setSetting(t, database, "TELEGRAM_CHAT_ID", "123")
	if tok, chat := s.telegramConfig(); tok != "tok" || chat != "123" {
		t.Errorf("telegramConfig = (%q,%q), want (tok,123)", tok, chat)
	}

	if s.webhookURL() != "" {
		t.Error("webhookURL should be empty when unset")
	}
	setSetting(t, database, "WEBHOOK_URL", "https://example.com/hook")
	if s.webhookURL() != "https://example.com/hook" {
		t.Errorf("webhookURL = %q", s.webhookURL())
	}
}

func TestSMTPConfig_RequiresHostAndTo_AndDefaults(t *testing.T) {
	database := testDB(t)
	s := New(database)

	if _, ok := s.smtpConfig(); ok {
		t.Error("smtpConfig should be incomplete with no host/to")
	}

	setSetting(t, database, "SMTP_HOST", "smtp.example.com")
	setSetting(t, database, "SMTP_TO", "ops@example.com")
	setSetting(t, database, "SMTP_USER", "user@example.com")
	cfg, ok := s.smtpConfig()
	if !ok {
		t.Fatal("smtpConfig should be complete with host + to")
	}
	if cfg.port != "587" {
		t.Errorf("port default = %q, want 587", cfg.port)
	}
	if cfg.from != "user@example.com" {
		t.Errorf("from should default to user, got %q", cfg.from)
	}
}

func TestSendWebhook(t *testing.T) {
	database := testDB(t)
	s := New(database)

	got := make(chan webhookPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		var p webhookPayload
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &p)
		got <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No URL configured → no-op (must not panic or block).
	s.sendWebhook(EventBackupDone, "noop")

	setSetting(t, database, "WEBHOOK_URL", srv.URL)
	s.sendWebhook(EventBackupFailed, "disk full")

	select {
	case p := <-got:
		if p.Event != EventBackupFailed || p.Message != "disk full" {
			t.Errorf("payload = %+v", p)
		}
		if p.Time == "" {
			t.Error("payload time should be set")
		}
	default:
		t.Fatal("webhook server received no request")
	}
}

func TestNotify_DisabledEventSendsNothing(t *testing.T) {
	database := testDB(t)
	s := New(database)
	setSetting(t, database, "NOTIFY_EVENTS", "none")
	// With all events disabled and no channels configured, Notify must be a
	// safe no-op (exercises the eventEnabled gate in Send).
	s.Notify(context.Background(), EventLoginFail, "title", "detail")
}
