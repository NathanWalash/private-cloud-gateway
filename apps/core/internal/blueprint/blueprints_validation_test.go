package blueprint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
)

// blueprintsDir is the repo's bundled blueprint directory, relative to this
// package (apps/core/internal/blueprint).
const blueprintsDir = "../../../../blueprints"

// imageTag returns the tag portion of an image reference, or "" if untagged.
// Handles registry hosts with ports (e.g. docker.n8n.io/n8nio/n8n:2.35.3).
func imageTag(image string) string {
	name := image
	if i := strings.LastIndex(image, "/"); i >= 0 {
		name = image[i+1:]
	}
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return ""
}

// TestBundledBlueprintsAreValid guards the whole blueprints/ directory against
// the classes of bug that have bitten this project: invalid schema, images that
// float on a mutable tag, and hard-coded dev domains that break in production.
func TestBundledBlueprintsAreValid(t *testing.T) {
	entries, err := os.ReadDir(blueprintsDir)
	if err != nil {
		t.Fatalf("read blueprints dir: %v", err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(blueprintsDir, e.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			// Must parse and pass schema validation (id, name, image, route...).
			bp, err := blueprint.Parse(data)
			if err != nil {
				t.Fatalf("parse/validate: %v", err)
			}

			// Image must be pinned to an explicit, immutable-ish tag.
			tag := imageTag(bp.Container.Image)
			if tag == "" {
				t.Errorf("image %q has no tag — pin it to a version", bp.Container.Image)
			}
			if tag == "latest" {
				t.Errorf("image %q uses :latest — pin it to a version for reproducible deploys", bp.Container.Image)
			}

			// No hard-coded dev domain: apps must use ${DOMAIN}/${SCHEME} so they
			// work in production. (This is the bug the templating fix addressed.)
			for _, env := range bp.Container.Environment {
				if strings.Contains(env, "localtest.me") {
					t.Errorf("env %q hard-codes the dev domain — use ${DOMAIN}/${SCHEME}", env)
				}
			}
		})
	}

	if count == 0 {
		t.Fatalf("no blueprints found in %s", blueprintsDir)
	}
}
