package actions

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"strings"
)

// RenderIconHalfBlocks converts raw 16x16 PNG bytes into terminal lines using unicode half-block characters (▀ and ▄)
// with 24-bit truecolor ANSI escape sequences. Each character represents 2 vertical pixels,
// so a 16x16 icon renders in 16 columns by 8 text rows.
func RenderIconHalfBlocks(pngData []byte) ([]string, error) {
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode icon png: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Normalize image size down or up to 16x16 if necessary
	targetW := 16
	targetH := 16
	var src image.Image = img
	if width != targetW || height != targetH {
		resized := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		for ty := 0; ty < targetH; ty++ {
			sy := bounds.Min.Y + (ty*height)/targetH
			for tx := 0; tx < targetW; tx++ {
				sx := bounds.Min.X + (tx*width)/targetW
				resized.Set(tx, ty, img.At(sx, sy))
			}
		}
		src = resized
		bounds = resized.Bounds()
		width = targetW
		height = targetH
	}

	var lines []string
	for y := bounds.Min.Y; y < bounds.Min.Y+height; y += 2 {
		var sb strings.Builder
		for x := bounds.Min.X; x < bounds.Min.X+width; x++ {
			r1, g1, b1, a1 := src.At(x, y).RGBA()
			r2, g2, b2, a2 := uint32(0), uint32(0), uint32(0), uint32(0)
			if y+1 < bounds.Min.Y+height {
				r2, g2, b2, a2 = src.At(x, y+1).RGBA()
			}

			// RGBA() returns alpha-premultiplied uint32 in [0, 0xffff]. Convert to 8-bit.
			fgR, fgG, fgB := r1>>8, g1>>8, b1>>8
			bgR, bgG, bgB := r2>>8, g2>>8, b2>>8

			hasTop := a1 > 0x8000
			hasBottom := a2 > 0x8000

			if hasTop && hasBottom {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m", fgR, fgG, fgB, bgR, bgG, bgB))
			} else if hasTop {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▀\x1b[0m", fgR, fgG, fgB))
			} else if hasBottom {
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▄\x1b[0m", bgR, bgG, bgB))
			} else {
				sb.WriteString(" ")
			}
		}
		lines = append(lines, sb.String())
	}

	return lines, nil
}
