package blueprint_test

import (
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

func TestValidateBlueprintID(t *testing.T) {
	valid := []string{
		"filebrowser",
		"uptime-kuma",
		"n8n",
		"actual-budget",
		"a",
		"app123",
	}
	for _, id := range valid {
		if err := blueprint.ValidateBlueprintID(id); err != nil {
			t.Errorf("ValidateBlueprintID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"",
		"../etc/passwd",
		"../../secret",
		"app name",     // space
		"App",          // uppercase
		"app.yaml",     // dot
		"app/subdir",   // slash
		"app;rm-rf",    // semicolon
		"a" + string(make([]byte, 65)), // too long
	}
	for _, id := range invalid {
		if err := blueprint.ValidateBlueprintID(id); err == nil {
			t.Errorf("ValidateBlueprintID(%q) expected error, got nil", id)
		}
	}
}

func TestParse_RejectsUnsafeID(t *testing.T) {
	yaml := `
id: "../etc/passwd"
name: Exploit
container:
  image: nginx
route:
  subdomain: test
  internal_port: 80
`
	_, err := blueprint.Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error parsing blueprint with path-traversal id, got nil")
	}
}
