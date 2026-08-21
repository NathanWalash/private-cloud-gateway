package docker

import (
	"bytes"
	"slices"
	"strings"
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

func TestNameHelpers(t *testing.T) {
	if got := appNetwork("immich"); got != "pcg-net-immich" {
		t.Errorf("appNetwork = %q", got)
	}
	if got := serviceContainerName("immich", "db"); got != "pcg-immich-db" {
		t.Errorf("serviceContainerName = %q", got)
	}
	if got := deriveAppID("pcg-immich"); got != "immich" {
		t.Errorf("deriveAppID = %q", got)
	}
}

func TestBuildCreateBody_MultiContainerNetworks(t *testing.T) {
	bp := &blueprint.Blueprint{
		ID:        "umami",
		Container: blueprint.Container{Image: "umami:latest"},
		Services: []blueprint.Service{
			{Name: "db", Image: "postgres:16"},
		},
	}
	body := buildCreateBody(bp)

	// App joins the per-app network (primary, so it resolves sidecars) AND the
	// shared network (so Caddy can reach it).
	if body.HostConfig.NetworkMode != appNetwork("umami") {
		t.Errorf("primary network = %q, want %q", body.HostConfig.NetworkMode, appNetwork("umami"))
	}
	if _, ok := body.NetworkingConfig.EndpointsConfig[appNetwork("umami")]; !ok {
		t.Error("app not attached to per-app network")
	}
	if _, ok := body.NetworkingConfig.EndpointsConfig[privateNetwork]; !ok {
		t.Error("app not attached to shared network")
	}
	if body.Labels["pcg.role"] != "app" {
		t.Errorf("app role label = %q", body.Labels["pcg.role"])
	}
}

func TestBuildCreateBody_SingleContainerUnchanged(t *testing.T) {
	bp := &blueprint.Blueprint{ID: "memos", Container: blueprint.Container{Image: "memos:latest"}}
	body := buildCreateBody(bp)
	if body.HostConfig.NetworkMode != privateNetwork {
		t.Errorf("single-container primary network = %q, want %q", body.HostConfig.NetworkMode, privateNetwork)
	}
	if len(body.NetworkingConfig.EndpointsConfig) != 1 {
		t.Errorf("single-container should join exactly one network, got %d", len(body.NetworkingConfig.EndpointsConfig))
	}
}

func TestBuildServiceCreateBody(t *testing.T) {
	body := buildServiceCreateBody("umami", blueprint.Service{
		Name: "db", Image: "postgres:16", Volumes: []string{"pcg-umami-db:/var/lib/postgresql/data"},
	})
	// Sidecar joins only the per-app network, aliased by its service name.
	if len(body.NetworkingConfig.EndpointsConfig) != 1 {
		t.Fatalf("service should join exactly one network, got %d", len(body.NetworkingConfig.EndpointsConfig))
	}
	ep := body.NetworkingConfig.EndpointsConfig[appNetwork("umami")]
	if ep == nil || !slices.Contains(ep.Aliases, "db") {
		t.Errorf("service missing 'db' alias on per-app network: %+v", ep)
	}
	if _, onShared := body.NetworkingConfig.EndpointsConfig[privateNetwork]; onShared {
		t.Error("sidecar must NOT be on the shared network")
	}
	if body.Labels["pcg.role"] != "service" {
		t.Errorf("service role label = %q", body.Labels["pcg.role"])
	}
	if !slices.Contains(body.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Error("service should get no-new-privileges by default")
	}
}

// frame builds one Docker multiplexed-stream frame (8-byte header + payload).
func frame(streamType byte, payload string) []byte {
	b := make([]byte, 8+len(payload))
	b[0] = streamType
	n := len(payload)
	b[4] = byte(n >> 24)
	b[5] = byte(n >> 16)
	b[6] = byte(n >> 8)
	b[7] = byte(n)
	copy(b[8:], payload)
	return b
}

func TestDemuxStream(t *testing.T) {
	var in []byte
	in = append(in, frame(1, "hello ")...)    // stdout
	in = append(in, frame(2, "a warning")...) // stderr
	in = append(in, frame(1, "world")...)     // stdout

	var stdout, stderr strings.Builder
	if err := demuxStream(bytes.NewReader(in), &stdout, &stderr); err != nil {
		t.Fatalf("demuxStream: %v", err)
	}
	if stdout.String() != "hello world" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hello world")
	}
	if stderr.String() != "a warning" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "a warning")
	}
}

func TestDemuxStream_Truncated(t *testing.T) {
	// A frame claiming 100 bytes but only 3 present must error, not hang/panic.
	f := frame(1, "")
	f[7] = 100
	f = append(f, []byte("abc")...)
	var out, errb strings.Builder
	if err := demuxStream(bytes.NewReader(f), &out, &errb); err == nil {
		t.Error("expected error on truncated frame")
	}
}

func TestDrainPullStream(t *testing.T) {
	// A normal progress stream ends cleanly.
	ok := `{"status":"Pulling from lib/x"}
{"status":"Download complete"}
`
	if err := drainPullStream(strings.NewReader(ok), "lib/x"); err != nil {
		t.Errorf("clean stream: unexpected error %v", err)
	}

	// Docker reports a failed pull as an {"error":...} object with HTTP 200 —
	// this MUST surface as an error (regression guard for silent pull failures).
	bad := `{"status":"Pulling"}
{"errorDetail":{"message":"manifest unknown"},"error":"manifest unknown"}
`
	err := drainPullStream(strings.NewReader(bad), "lib/nope")
	if err == nil {
		t.Fatal("expected error from a failed pull stream")
	}
	if !strings.Contains(err.Error(), "manifest unknown") {
		t.Errorf("error should include the docker message, got %v", err)
	}
}
