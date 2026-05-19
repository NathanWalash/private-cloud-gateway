package blueprint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

// TestCatalogue validates every .yaml file in the blueprints/ directory.
// Run from the repo root: go test ./internal/blueprint/ -run TestCatalogue
// This catches broken image tags, missing fields, and invalid YAML before deployment.
func TestCatalogue(t *testing.T) {
	// Walk up from apps/core to find blueprints/
	dir := findBlueprintsDir(t)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read blueprints dir %s: %v", dir, err)
	}

	var yamlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			yamlFiles = append(yamlFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(yamlFiles) == 0 {
		t.Skip("no blueprint files found — skipping catalogue test")
	}

	t.Logf("validating %d blueprints", len(yamlFiles))

	for _, path := range yamlFiles {
		path := path
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}

			bp, err := blueprint.Parse(data)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}

			// Filename must match blueprint ID (installer looks up {id}.yaml)
			expectedFile := bp.ID + ".yaml"
			if name != expectedFile {
				t.Errorf("filename %q does not match blueprint id %q — installer looks for %q",
					name, bp.ID, expectedFile)
			}

			// Image must not be empty or a placeholder
			if bp.Container.Image == "" {
				t.Errorf("container.image is empty")
			}
			if strings.Contains(bp.Container.Image, "example") || strings.Contains(bp.Container.Image, "TODO") {
				t.Errorf("container.image %q looks like a placeholder", bp.Container.Image)
			}

			// Internal port must be > 0
			if bp.Route.InternalPort <= 0 || bp.Route.InternalPort > 65535 {
				t.Errorf("route.internal_port %d is invalid", bp.Route.InternalPort)
			}

			// Subdomain must be non-empty and URL-safe
			if bp.Route.Subdomain == "" {
				t.Errorf("route.subdomain is empty")
			}
			for _, c := range bp.Route.Subdomain {
				if !isAlphaNum(c) && c != '-' {
					t.Errorf("route.subdomain %q contains invalid character %q", bp.Route.Subdomain, string(c))
					break
				}
			}
		})
	}
}

func findBlueprintsDir(t *testing.T) string {
	t.Helper()
	// Walk up from apps/core looking for blueprints/
	dir, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "blueprints")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Skip("blueprints/ directory not found relative to test location")
	return ""
}

func isAlphaNum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
