package main

import (
	"embed"
	"io/fs"
)

// web/dist is a placeholder (just .gitkeep) unless build.sh has copied the
// built frontend in first -- plain `go build .` still works either way,
// it just serves an empty directory at `/` until the frontend is built.
//
//go:embed all:web/dist
var embeddedWeb embed.FS

func webFS() fs.FS {
	sub, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		return embeddedWeb
	}
	return sub
}
