package util

import (
	"testing"
)

func TestMimeTypeByPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"image.PNG", "image/png"},
		{"photo.webp", "image/webp"},
		{"animation.gif", "image/gif"},
		{"audio.ogg", "audio/ogg"},
		{"audio.oga", "audio/ogg"},
		{"doc.pdf", "application/octet-stream"},
		{"noext", "application/octet-stream"},
		{"", "application/octet-stream"},
		{"/path/to/file.mp3", "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := MimeTypeByPath(tt.path); got != tt.want {
				t.Errorf("MimeTypeByPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
