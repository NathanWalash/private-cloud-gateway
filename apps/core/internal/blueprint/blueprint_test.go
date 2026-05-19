package blueprint_test

import (
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

const validYAML = `
id: test-app
name: Test App
description: A test application
icon: "🧪"
category: testing

route:
  subdomain: test
  internal_port: 8080

container:
  image: nginx:alpine

lifecycle:
  policy: always-on

health:
  path: /
  expected_status: 200
  timeout_seconds: 10

backup:
  enabled: false
`

func TestParse_Valid(t *testing.T) {
	bp, err := blueprint.Parse([]byte(validYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if bp.ID != "test-app" {
		t.Errorf("ID: got %q, want test-app", bp.ID)
	}
	if bp.Route.Subdomain != "test" {
		t.Errorf("Subdomain: got %q, want test", bp.Route.Subdomain)
	}
	if bp.Route.InternalPort != 8080 {
		t.Errorf("InternalPort: got %d, want 8080", bp.Route.InternalPort)
	}
	if bp.ContainerName() != "pcg-test-app" {
		t.Errorf("ContainerName: got %q, want pcg-test-app", bp.ContainerName())
	}
}

func TestParse_MissingID(t *testing.T) {
	_, err := blueprint.Parse([]byte(`
name: Missing ID App
container:
  image: nginx:alpine
route:
  subdomain: test
  internal_port: 8080
`))
	if err == nil {
		t.Error("expected error for missing id, got nil")
	}
}

func TestParse_MissingImage(t *testing.T) {
	_, err := blueprint.Parse([]byte(`
id: no-image
name: No Image
route:
  subdomain: test
  internal_port: 8080
`))
	if err == nil {
		t.Error("expected error for missing container.image, got nil")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := blueprint.Parse([]byte(`{not: valid: yaml: [`))
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}
