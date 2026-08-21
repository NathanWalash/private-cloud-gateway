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

// imageRefRegex restricts container images to the safe character set of a Docker
// image reference (registry/repo:tag@digest). It blocks spaces and URL
// metacharacters (?, &, #, …) that would otherwise be interpolated raw into the
// Docker Engine /images/create URL and could inject query params or a rogue
// registry.
var imageRefRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)

// volumeSourceRegex matches a Docker *named volume* (no path separators). Bind
// mounts of host paths are rejected so a blueprint can't mount the host root or
// the Docker socket into an app container.
var volumeSourceRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// serviceNameRegex constrains sidecar service names — they become a Docker
// network alias (the hostname the app uses) and part of the container name.
var serviceNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

// Blueprint defines an installable app.
type Blueprint struct {
	ID          string    `yaml:"id"`
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Icon        string    `yaml:"icon"`
	Category    string    `yaml:"category"`
	DependsOn   []string  `yaml:"depends_on"`  // blueprint IDs that must be running before this installs
	ComingSoon  bool      `yaml:"coming_soon"` // listed in the marketplace but not installable yet
	Route       Route     `yaml:"route"`
	Container   Container `yaml:"container"`
	Services    []Service `yaml:"services"` // private sidecar containers (db/redis/…) created with the app
	Lifecycle   Lifecycle `yaml:"lifecycle"`
	Health      Health    `yaml:"health"`
	Backup      Backup    `yaml:"backup"`
	Resources   Resources `yaml:"resources"`
}

// Service is a private sidecar container (e.g. Postgres, Redis) created and torn
// down with the app on its per-app network. The app reaches it by Name as
// hostname. Sidecars are never exposed through Caddy.
type Service struct {
	Name        string        `yaml:"name"`
	Image       string        `yaml:"image"`
	Environment []string      `yaml:"environment"`
	Volumes     []string      `yaml:"volumes"`
	Security    Security      `yaml:"security"`
	MemoryLimit string        `yaml:"memory_limit"`
	Backup      ServiceBackup `yaml:"backup"`
}

// ServiceBackup declares how to dump/restore a stateful service (a database).
// Both are argv run inside the service container: dump's stdout is archived,
// restore receives the dump on stdin. Omit for stateless services (e.g. Redis).
type ServiceBackup struct {
	Dump    []string `yaml:"dump"`
	Restore []string `yaml:"restore"`
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

// weakSecretValues are placeholder/default secret values that must be changed
// before an app is exposed to the internet.
var weakSecretValues = map[string]bool{
	"changeme": true, "change-me": true, "changethis": true, "change-this": true,
	"password": true, "passwd": true, "admin": true, "secret": true,
	"default": true, "test": true, "123456": true, "root": true, "example": true,
}

// secretKeyHint matches environment variable names that hold a secret.
var secretKeyHint = regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key|access_key|_key)`)

// WeakSecrets scans the container environment for secret-looking variables whose
// value is a known weak default (e.g. COUCHDB_PASSWORD=changeme) and returns a
// human-readable message per finding. Callers should warn the operator so the
// value gets changed before the app faces the internet. Render the blueprint
// first so ${DOMAIN}/${SCHEME} are already substituted.
func (bp *Blueprint) WeakSecrets() []string {
	var out []string
	scan := func(where string, env []string) {
		for _, e := range env {
			k, v, ok := strings.Cut(e, "=")
			if !ok {
				continue
			}
			if secretKeyHint.MatchString(k) && weakSecretValues[strings.ToLower(strings.TrimSpace(v))] {
				out = append(out, fmt.Sprintf("%s%s uses a weak default value (%q) — change it before exposing this app", where, k, v))
			}
		}
	}
	scan("", bp.Container.Environment)
	for _, s := range bp.Services {
		scan("services."+s.Name+".", s.Environment)
	}
	return out
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

// ValidServiceName reports whether name is a safe sidecar service name (used in
// container names and network aliases).
func ValidServiceName(name string) bool {
	return serviceNameRegex.MatchString(name)
}

// validateVolumes rejects host-path bind mounts; only named volumes are allowed.
func validateVolumes(field string, volumes []string) []error {
	var errs []error
	for _, v := range volumes {
		src := v
		if i := strings.Index(v, ":"); i >= 0 {
			src = v[:i]
		}
		if !volumeSourceRegex.MatchString(src) {
			errs = append(errs, fmt.Errorf("%s: %q must use a named volume, not a host path", field, v))
		}
	}
	return errs
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
	} else if !imageRefRegex.MatchString(bp.Container.Image) {
		errs = append(errs, fmt.Errorf("container.image %q contains invalid characters", bp.Container.Image))
	}
	// Only named volumes are allowed — reject host-path bind mounts (e.g.
	// "/:/host" or the docker socket), which would let an app escape onto the host.
	errs = append(errs, validateVolumes("container.volumes", bp.Container.Volumes)...)

	// Sidecar services: safe name, valid image, named volumes only.
	seen := map[string]bool{}
	for i, s := range bp.Services {
		if !serviceNameRegex.MatchString(s.Name) {
			errs = append(errs, fmt.Errorf("services[%d].name %q is invalid (lowercase letters, digits, hyphens)", i, s.Name))
		}
		if seen[s.Name] {
			errs = append(errs, fmt.Errorf("services: duplicate service name %q", s.Name))
		}
		seen[s.Name] = true
		if s.Image == "" {
			errs = append(errs, fmt.Errorf("services[%d].image is required", i))
		} else if !imageRefRegex.MatchString(s.Image) {
			errs = append(errs, fmt.Errorf("services[%d].image %q contains invalid characters", i, s.Image))
		}
		errs = append(errs, validateVolumes(fmt.Sprintf("services[%d].volumes", i), s.Volumes)...)
	}
	switch bp.Route.Subdomain {
	case "":
		errs = append(errs, errors.New("route.subdomain is required"))
	case "home":
		// "home" is reserved for the dashboard — a blueprint using it would emit
		// a duplicate Caddy site block and shadow/break the dashboard route.
		errs = append(errs, errors.New(`route.subdomain "home" is reserved for the dashboard`))
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
