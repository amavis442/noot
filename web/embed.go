// Package web holds the React frontend. Its build output, web/dist, is
// embedded in the noot binary and served by `noot serve`. Without a build the
// folder only holds .gitkeep and the server shows a placeholder page.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend files, index.html at the root.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a valid path, so fs.Sub cannot fail
	}
	return sub
}
