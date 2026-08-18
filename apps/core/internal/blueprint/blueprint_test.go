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

func TestRenderSubstitutesDomainAndScheme(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID:   "n8n",
		Name: "n8n",
		Container: blueprint.Container{
			Image: "n8nio/n8n:latest",
			Environment: []string{
				"N8N_HOST=n8n.${DOMAIN}",
				"N8N_PROTOCOL=${SCHEME}",
				"WEBHOOK_URL=${SCHEME}://n8n.${DOMAIN}/",
				"NODE_ENV=production", // no placeholders — must be untouched
			},
		},
	}

	got := bp.Render("example.com", "https")

	want := []string{
		"N8N_HOST=n8n.example.com",
		"N8N_PROTOCOL=https",
		"WEBHOOK_URL=https://n8n.example.com/",
		"NODE_ENV=production",
	}
	for i, w := range want {
		if got.Container.Environment[i] != w {
			t.Errorf("env[%d] = %q, want %q", i, got.Container.Environment[i], w)
		}
	}
}

func TestRenderDoesNotMutateOriginal(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID:        "x",
		Name:      "x",
		Container: blueprint.Container{Image: "x", Environment: []string{"HOST=${DOMAIN}"}},
	}
	_ = bp.Render("example.com", "https")
	if bp.Container.Environment[0] != "HOST=${DOMAIN}" {
		t.Errorf("original mutated: %q", bp.Container.Environment[0])
	}
}
