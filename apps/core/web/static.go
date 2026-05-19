// Package web embeds the compiled Vite dashboard app.
// The dist/ directory is either the committed placeholder or the real
// build output produced by the Docker multi-stage build.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS returns the embedded dist directory as an fs.FS for use with http.FileServer.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: failed to sub dist: " + err.Error())
	}
	return sub
}
