//go:build prod

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var frontendFS embed.FS

func getStaticFS() fs.FS {
	staticFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		return nil
	}
	return staticFS
}
