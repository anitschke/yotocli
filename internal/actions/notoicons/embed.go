//go:generate go run ../../tools/gennoto

package notoicons

import "embed"

// FS embeds all 32x32 Noto Emoji PNG images.
//
//go:embed images/*.png
var FS embed.FS
