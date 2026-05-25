package util

import (
	"path/filepath"
	"strings"
)

func MimeTypeByPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".ogg", ".oga":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
