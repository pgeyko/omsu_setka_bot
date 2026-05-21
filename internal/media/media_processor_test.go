package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// createTestJPEG creates a synthetic JPEG of the given dimensions.
func createTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a gradient so the image has actual content.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("createTestJPEG: %v", err)
	}
	return buf.Bytes()
}

// createTestPNG creates a synthetic PNG of the given dimensions.
func createTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("createTestPNG: %v", err)
	}
	return buf.Bytes()
}

// decodeDimensions decodes the dimensions from a JPEG byte slice.
func decodeDimensions(t *testing.T, data []byte) (int, int) {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decodeDimensions: %v", err)
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

func TestResizeIfNeeded_AlreadySmall(t *testing.T) {
	// 200×150 is within 512px — should pass through (re-encoded as JPEG).
	input := createTestJPEG(t, 200, 150)
	out, mime, err := resizeIfNeeded(input, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
	w, h := decodeDimensions(t, out)
	if w > maxPhotoDimension || h > maxPhotoDimension {
		t.Errorf("expected dims ≤ %d, got %dx%d", maxPhotoDimension, w, h)
	}
}

func TestResizeIfNeeded_WideImage(t *testing.T) {
	// 1024×300: wide image, should be scaled down to 512×150.
	input := createTestJPEG(t, 1024, 300)
	out, mime, err := resizeIfNeeded(input, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mime)
	}
	w, h := decodeDimensions(t, out)
	if w > maxPhotoDimension {
		t.Errorf("width %d exceeds max %d", w, maxPhotoDimension)
	}
	// Height should be proportional: 300 * 512/1024 = 150.
	if h < 140 || h > 160 {
		t.Errorf("expected height ~150, got %d", h)
	}
}

func TestResizeIfNeeded_TallImage(t *testing.T) {
	// 300×1024: tall image, should be scaled to 150×512.
	input := createTestJPEG(t, 300, 1024)
	out, _, err := resizeIfNeeded(input, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, h := decodeDimensions(t, out)
	if h > maxPhotoDimension {
		t.Errorf("height %d exceeds max %d", h, maxPhotoDimension)
	}
	if w < 140 || w > 160 {
		t.Errorf("expected width ~150, got %d", w)
	}
}

func TestResizeIfNeeded_PNG(t *testing.T) {
	// PNG input, 800×600 — should be resized and re-encoded as JPEG.
	input := createTestPNG(t, 800, 600)
	out, mime, err := resizeIfNeeded(input, "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("expected output mime image/jpeg, got %s", mime)
	}
	w, h := decodeDimensions(t, out)
	if w > maxPhotoDimension || h > maxPhotoDimension {
		t.Errorf("expected dims ≤ %d, got %dx%d", maxPhotoDimension, w, h)
	}
}

func TestResizeIfNeeded_SquareExactBoundary(t *testing.T) {
	// Exactly 512×512 — should pass through without upscaling.
	input := createTestJPEG(t, 512, 512)
	out, _, err := resizeIfNeeded(input, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, h := decodeDimensions(t, out)
	if w > maxPhotoDimension || h > maxPhotoDimension {
		t.Errorf("expected dims ≤ %d, got %dx%d", maxPhotoDimension, w, h)
	}
}

func TestGetMimeTypeByPath(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{"file.jpg", "image/jpeg"},
		{"file.JPEG", "image/jpeg"},
		{"photo.png", "image/png"},
		{"voice.ogg", "audio/ogg"},
		{"voice.oga", "audio/ogg"},
		{"image.webp", "image/webp"},
		{"image.gif", "image/gif"},
		{"unknown.bin", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}
	for _, tc := range cases {
		got := getMimeTypeByPath(tc.path)
		if got != tc.expected {
			t.Errorf("getMimeTypeByPath(%q) = %q, want %q", tc.path, got, tc.expected)
		}
	}
}
