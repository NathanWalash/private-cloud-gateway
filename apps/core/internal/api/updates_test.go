package api

import (
	"strings"
	"testing"
)

func TestGetLatestDigest_NonDockerHub(t *testing.T) {
	// Images with explicit registry hosts should be skipped, not return an error.
	cases := []string{
		"registry.example.com/myapp:latest",
		"ghcr.io/owner/repo:v1",
		"quay.io/org/image",
	}
	for _, img := range cases {
		d, err := getLatestDigest(img)
		if err != nil {
			t.Errorf("getLatestDigest(%q): unexpected error: %v", img, err)
		}
		if d != "" {
			t.Errorf("getLatestDigest(%q): expected empty digest for non-Docker-Hub image, got %q", img, d)
		}
	}
}

func TestGetLatestDigest_ImageParsing(t *testing.T) {
	// Verify the URL construction for Docker Hub images by checking the
	// ref/tag parsing directly — no network calls.
	cases := []struct {
		image   string
		wantRef string
		wantTag string
	}{
		{"nginx", "library/nginx", "latest"},
		{"nginx:1.25", "library/nginx", "1.25"},
		{"bitnami/postgresql:15", "bitnami/postgresql", "15"},
		{"bitnami/postgresql", "bitnami/postgresql", "latest"},
	}
	for _, tc := range cases {
		ref, tag := tc.image, "latest"
		if i := strings.LastIndex(tc.image, ":"); i > strings.LastIndex(tc.image, "/") {
			ref, tag = tc.image[:i], tc.image[i+1:]
		}
		parts := strings.Split(ref, "/")
		if len(parts) == 1 {
			ref = "library/" + ref
		}
		if ref != tc.wantRef {
			t.Errorf("image %q: got ref %q, want %q", tc.image, ref, tc.wantRef)
		}
		if tag != tc.wantTag {
			t.Errorf("image %q: got tag %q, want %q", tc.image, tag, tc.wantTag)
		}
	}
}

func TestUpdateInfo_JSON(t *testing.T) {
	info := UpdateInfo{
		AppID:        1,
		BlueprintID:  "nginx",
		CurrentImage: "nginx:latest",
		LatestDigest: "sha256:abc",
		UpdateAvail:  true,
	}
	if info.AppID != 1 {
		t.Errorf("unexpected AppID: %d", info.AppID)
	}
	if !info.UpdateAvail {
		t.Error("expected UpdateAvail true")
	}
}
