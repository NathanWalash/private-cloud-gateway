// Package version exposes the build version of Cloud Core.
//
// The default is "dev". Release builds override it at link time:
//
//	go build -ldflags="-X github.com/NathanWalash/private-cloud-gateway/apps/core/internal/version.Version=v1.2.3"
package version

// Version is the running build's version. Overridden via -ldflags in release
// builds; "dev" for local/unstamped builds.
var Version = "dev"
