package domain

import (
	"path"
	"strings"
)

// IsImage reports whether the file at p is a raster image a browser can show
// safely. SVG is not one: it can carry scripts.
func IsImage(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".bmp", ".ico":
		return true
	}
	return false
}
