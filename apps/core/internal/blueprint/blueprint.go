// Package blueprint handles parsing and validation of YAML app blueprints.
package blueprint

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
	DependsOn   []string  `yaml:"depends_on"` // blueprint IDs that must be running before this installs
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
	Security    Security `yaml:"security"`
}

// Security controls container hardening. The zero value is the secure default:
// privilege escalation is blocked (no-new-privileges). Root filesystem and
// capabilities stay permissive unless a blueprint opts in, because many
// third-party images write to their root FS or need specific capabilities.
type Security struct {
	// AllowPrivilegeEscalation, when true, disables the no-new-privileges flag.
	// Default false → no-new-privileges is ON. Web apps almost never need setuid.
	AllowPrivilegeEscalation bool `yaml:"allow_privilege_escalation"`
	// ReadOnlyRootfs mounts the container's root filesystem read-only.
	ReadOnlyRootfs bool `yaml:"read_only_rootfs"`
	// CapDrop / CapAdd tune Linux capabilities (e.g. CapDrop: ["ALL"]).
	CapDrop []string `yaml:"cap_drop"`
	CapAdd  []string `yaml:"cap_add"`
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

// Render returns a copy of the blueprint with deployment placeholders in
// container environment values substituted for the running instance's values:
//
//	${DOMAIN}  -> the configured root domain (e.g. example.com)
//	${SCHEME}  -> "https" in production, "http" in local dev
//
// This lets a single blueprint work in both dev (localtest.me/http) and
// production (real domain/https) without hard-coding either.
func (bp *Blueprint) Render(domain, scheme string) *Blueprint {
	out := *bp
	if len(bp.Container.Environment) > 0 {
		repl := strings.NewReplacer("${DOMAIN}", domain, "${SCHEME}", scheme)
		env := make([]string, len(bp.Container.Environment))
		for i, e := range bp.Container.Environment {
			env[i] = repl.Replace(e)
		}
		out.Container.Environment = env
	}
	return &out
}
