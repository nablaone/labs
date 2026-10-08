// Package web embeds the browser client (vanilla JS, no build step).
package web

import (
	"embed"
	"io/fs"
	"mime"
)

func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

//go:embed static
var files embed.FS

// Static returns the client files rooted at static/.
func Static() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
