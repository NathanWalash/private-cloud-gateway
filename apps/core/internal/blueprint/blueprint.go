// Package blueprint handles parsing and validation of YAML app blueprints.
package blueprint

import (
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// blueprintIDRegex enforces safe blueprint IDs: lowercase letters, digits, hyphens only.
// This prevents path traversal (../../etc/passwd) and Docker name injection.
var blueprintIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Blueprint defines an installable app.
type Blueprint struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Icon        string    `yaml:"icon"`
	Category    string    `yaml:"category"`
	Route       Route     `yaml:"route"`
	Container   Container `yaml:"container"`
	Lifecycle   Lifecycle `yaml:"lifecycle"`
	Health      Health    `yaml:"health"`
	Backup      Backup    `yaml:"backup"`
	Resources   Resources `yaml:"resources"`
}

type Route struct {
	Subdomain    string `yaml:"subdomain"`
	InternalPort int    `yaml:"internal_port"`
}

type Container struct {
	Image       string   `yaml:"image"`
	Environment []string `yaml:"environment"`
	Volumes     []string `yaml:"volumes"`
}

type Lifecycle struct {
	// Policy is "always-on" or "scale-to-zero".
	Policy string `yaml:"policy"`
}

type Health struct {
	Path           string `yaml:"path"`
	ExpectedStatus int    `yaml:"expected_status"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type Backup struct {
	Enabled        bool     `yaml:"enabled"`
	Paths          []string `yaml:"paths"`           // legacy host paths
	ContainerPaths []string `yaml:"container_paths"` // paths inside the container to archive
}

type Resources struct {
	MemoryLimit string `yaml:"memory_limit"`
}

// Parse decodes YAML blueprint data and validates it.
func Parse(data []byte) (*Blueprint, error) {
	var bp Blueprint
	if err := yaml.Unmarshal(data, &bp); err != nil {
		return nil, fmt.Errorf("parse blueprint yaml: %w", err)
	}
	if err := bp.Validate(); err != nil {
		return nil, err
	}
	return &bp, nil
}

// ValidateBlueprintID returns an error if id contains unsafe characters.
// Call this on any user-supplied blueprint ID before using it in file paths.
func ValidateBlueprintID(id string) error {
	if !blueprintIDRegex.MatchString(id) {
		return fmt.Errorf("blueprint id %q is invalid: must match %s", id, blueprintIDRegex)
	}
	return nil
}

// Validate checks that required fields are present and safe.
func (bp *Blueprint) Validate() error {
	var errs []error
	if bp.ID == "" {
		errs = append(errs, errors.New("id is required"))
	} else if !blueprintIDRegex.MatchString(bp.ID) {
		errs = append(errs, fmt.Errorf("id %q contains invalid characters — use lowercase letters, digits, hyphens only", bp.ID))
	}
	if bp.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if bp.Container.Image == "" {
		errs = append(errs, errors.New("container.image is required"))
	}
	if bp.Route.Subdomain == "" {
		errs = append(errs, errors.New("route.subdomain is required"))
	}
	if bp.Route.InternalPort == 0 {
		errs = append(errs, errors.New("route.internal_port is required"))
	}
	return errors.Join(errs...)
}

// ContainerName returns the Docker container name for this blueprint.
func (bp *Blueprint) ContainerName() string {
	return "pcg-" + bp.ID
}
