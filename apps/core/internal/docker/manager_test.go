package docker

import (
	"slices"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

func TestBuildCreateBody_SecureDefaults(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID: "notes",
		Container: blueprint.Container{
			Image:   "example/notes:latest",
			Volumes: []string{"notes_data:/data"},
		},
	}
	body := buildCreateBody(bp)

	// no-new-privileges is on by default.
	if !slices.Contains(body.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Errorf("expected no-new-privileges by default, got %v", body.HostConfig.SecurityOpt)
	}
	// Permissive defaults left untouched.
	if body.HostConfig.ReadonlyRootfs {
		t.Error("root filesystem should be writable by default")
	}
	if len(body.HostConfig.CapDrop) != 0 || len(body.HostConfig.CapAdd) != 0 {
		t.Error("capabilities should be unchanged by default")
	}
	// Core wiring preserved.
	if body.HostConfig.NetworkMode != privateNetwork {
		t.Errorf("expected private network, got %q", body.HostConfig.NetworkMode)
	}
	if !slices.Equal(body.HostConfig.Binds, []string{"notes_data:/data"}) {
		t.Errorf("binds not preserved: %v", body.HostConfig.Binds)
	}
	if body.Labels["pcg.managed"] != "true" {
		t.Error("managed label missing")
	}
}

func TestBuildCreateBody_HardenedBlueprint(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID: "locked",
		Container: blueprint.Container{
			Image: "example/locked:latest",
			Security: blueprint.Security{
				ReadOnlyRootfs: true,
				CapDrop:        []string{"ALL"},
				CapAdd:         []string{"NET_BIND_SERVICE"},
			},
		},
	}
	body := buildCreateBody(bp)

	if !body.HostConfig.ReadonlyRootfs {
		t.Error("read-only rootfs not applied")
	}
	if !slices.Equal(body.HostConfig.CapDrop, []string{"ALL"}) {
		t.Errorf("cap_drop not applied: %v", body.HostConfig.CapDrop)
	}
	if !slices.Equal(body.HostConfig.CapAdd, []string{"NET_BIND_SERVICE"}) {
		t.Errorf("cap_add not applied: %v", body.HostConfig.CapAdd)
	}
	if !slices.Contains(body.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Error("no-new-privileges should still be on")
	}
}

func TestBuildCreateBody_AllowPrivilegeEscalation(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID: "legacy",
		Container: blueprint.Container{
			Image:    "example/legacy:latest",
			Security: blueprint.Security{AllowPrivilegeEscalation: true},
		},
	}
	body := buildCreateBody(bp)

	if slices.Contains(body.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Error("no-new-privileges should be off when escalation is explicitly allowed")
	}
}
