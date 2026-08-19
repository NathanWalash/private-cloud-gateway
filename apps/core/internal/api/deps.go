package api

import (
	"context"
	"io"

	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/blueprint"
	"github.com/NathanWalash/private-cloud-gateway/apps/core/internal/caddy"
)

// dockerManager is the subset of *docker.Manager the API depends on. Defining it
// on the consumer side lets handlers be unit-tested with a fake, and keeps the
// coupling explicit.
type dockerManager interface {
	Install(ctx context.Context, bp *blueprint.Blueprint) error
	Start(ctx context.Context, containerName string) error
	Stop(ctx context.Context, containerName string) error
	Restart(ctx context.Context, containerName string) error
	Remove(ctx context.Context, containerName string) error
	StatusAfterStart(ctx context.Context, containerName string, maxSeconds int) string
	Logs(ctx context.Context, containerName string, tail int) (string, error)
	LogsFollow(ctx context.Context, containerName string) (io.ReadCloser, error)
	UpdateImage(ctx context.Context, image string) error
	CopyFromContainer(ctx context.Context, containerName, srcPath string) (io.ReadCloser, error)
}

// caddyManager is the subset of *caddy.Manager the API depends on.
type caddyManager interface {
	ReloadAll(ctx context.Context, apps []caddy.AppRoute) error
}
