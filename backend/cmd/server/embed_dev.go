//go:build !prod

package main

import "io/fs"

func getStaticFS() fs.FS {
	return nil
}
