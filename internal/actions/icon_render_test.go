package actions

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func createTestPNG(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	// Set some pixels
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(0, 1, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestRenderIconHalfBlocks(t *testing.T) {
	pngData := createTestPNG(t)
	lines, err := RenderIconHalfBlocks(pngData)
	if err != nil {
		t.Fatalf("RenderIconHalfBlocks failed: %v", err)
	}

	// 16 rows high with 2 pixels per row -> 8 text lines
	if len(lines) != 8 {
		t.Errorf("Expected 8 lines of half-block render, got %d", len(lines))
	}

	// Line 0 should have escape codes containing RGB values for (255, 0, 0) and (0, 255, 0)
	if !strings.Contains(lines[0], "255;0;0") {
		t.Errorf("Expected top pixel red (255,0,0) in line 0, got %s", lines[0])
	}
	if !strings.Contains(lines[0], "0;255;0") {
		t.Errorf("Expected bottom pixel green (0,255,0) in line 0, got %s", lines[0])
	}
}
