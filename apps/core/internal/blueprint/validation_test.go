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
		"app name",                     // space
		"App",                          // uppercase
		"app.yaml",                     // dot
		"app/subdir",                   // slash
		"app;rm-rf",                    // semicolon
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

func bpYAML(image, volume string) string {
	vols := "[]"
	if volume != "" {
		vols = "[" + volume + "]"
	}
	return "id: testapp\nname: Test\ncontainer:\n  image: " + image +
		"\n  volumes: " + vols + "\nroute:\n  subdomain: test\n  internal_port: 80\n"
}

func TestParse_RejectsMaliciousImage(t *testing.T) {
	bad := []string{
		"evil image",          // space
		"repo:tag?injected=1", // query injection
		"repo:tag&x=1",        // ampersand
		"repo:tag#frag",       // fragment
		`repo";rm -rf`,        // shell-ish junk
	}
	for _, img := range bad {
		if _, err := blueprint.Parse([]byte(bpYAML(img, ""))); err == nil {
			t.Errorf("expected image %q to be rejected", img)
		}
	}
	good := []string{
		"excalidraw/excalidraw:sha-4bfc5bb",
		"ghcr.io/home-assistant/home-assistant:stable",
		"couchdb:3.5.2",
		"nginx",
	}
	for _, img := range good {
		if _, err := blueprint.Parse([]byte(bpYAML(img, ""))); err != nil {
			t.Errorf("expected image %q to be accepted, got %v", img, err)
		}
	}
}

func TestParse_RejectsHostPathVolume(t *testing.T) {
	bad := []string{
		`"/:/host"`,                 // host root
		`"/var/run/docker.sock:/x"`, // docker socket
		`"./data:/data"`,            // relative host path
		`"../escape:/data"`,         // traversal
	}
	for _, v := range bad {
		if _, err := blueprint.Parse([]byte(bpYAML("nginx", v))); err == nil {
			t.Errorf("expected volume %s to be rejected", v)
		}
	}
	// Named volumes are allowed.
	if _, err := blueprint.Parse([]byte(bpYAML("nginx", `"pcg-data:/opt/data"`))); err != nil {
		t.Errorf("expected named volume to be accepted, got %v", err)
	}
}

func TestParse_RejectsReservedHomeSubdomain(t *testing.T) {
	yaml := `
id: homeassistant
name: Home Assistant
container:
  image: ghcr.io/home-assistant/home-assistant:stable
route:
  subdomain: home
  internal_port: 8123
`
	_, err := blueprint.Parse([]byte(yaml))
	if err == nil {
		t.Error(`expected error for reserved "home" subdomain, got nil`)
	}
}

func TestParse_Services(t *testing.T) {
	valid := `
id: umami
name: Umami
container:
  image: umami:latest
services:
  - name: db
    image: postgres:16-alpine
    volumes: ["pcg-umami-db:/var/lib/postgresql/data"]
route:
  subdomain: analytics
  internal_port: 3000
`
	if _, err := blueprint.Parse([]byte(valid)); err != nil {
		t.Errorf("valid services blueprint rejected: %v", err)
	}

	badName := `
id: umami
name: Umami
container:
  image: umami:latest
services:
  - name: "Bad Name"
    image: postgres:16
route:
  subdomain: analytics
  internal_port: 3000
`
	if _, err := blueprint.Parse([]byte(badName)); err == nil {
		t.Error("expected invalid service name to be rejected")
	}

	hostVol := `
id: umami
name: Umami
container:
  image: umami:latest
services:
  - name: db
    image: postgres:16
    volumes: ["/var/run/docker.sock:/x"]
route:
  subdomain: analytics
  internal_port: 3000
`
	if _, err := blueprint.Parse([]byte(hostVol)); err == nil {
		t.Error("expected host-path service volume to be rejected")
	}
}
