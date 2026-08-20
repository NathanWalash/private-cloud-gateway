package blueprint_test

import (
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

func TestWeakSecrets(t *testing.T) {
	bp := &blueprint.Blueprint{
		Container: blueprint.Container{
			Environment: []string{
				"COUCHDB_USER=admin",            // not a secret key → ignored
				"COUCHDB_PASSWORD=changeme",     // weak → flagged
				"API_TOKEN=Xr9-strong-value-42", // strong → ok
				"JWT_SECRET=secret",             // weak → flagged
				"NODE_ENV=production",           // not a secret key → ignored
				"MALFORMED_ENTRY",               // no '=' → ignored
			},
		},
	}
	found := bp.WeakSecrets()
	if len(found) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(found), found)
	}
	joined := strings.Join(found, "\n")
	if !strings.Contains(joined, "COUCHDB_PASSWORD") {
		t.Error("expected COUCHDB_PASSWORD to be flagged")
	}
	if !strings.Contains(joined, "JWT_SECRET") {
		t.Error("expected JWT_SECRET to be flagged")
	}
}

func TestWeakSecrets_NoneWhenAllStrong(t *testing.T) {
	bp := &blueprint.Blueprint{
		Container: blueprint.Container{
			Environment: []string{
				"DB_PASSWORD=k3+long-random-9f",
				"SESSION_SECRET=a1b2c3d4e5f6a7b8",
			},
		},
	}
	if found := bp.WeakSecrets(); len(found) != 0 {
		t.Errorf("expected no findings, got %v", found)
	}
}
