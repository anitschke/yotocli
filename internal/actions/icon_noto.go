package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/vgaro/yotocli/internal/actions/notoicons"
)

// NotoEmojiIcon implements Icon for Noto Emoji icons embedded in the binary.
type NotoEmojiIcon struct {
	entry notoicons.NotoEmojiEntry

	mu     sync.Mutex
	data   []byte
	sha256 string
}

func NewNotoEmojiIcon(entry notoicons.NotoEmojiEntry) *NotoEmojiIcon {
	return &NotoEmojiIcon{
		entry: entry,
	}
}

// ID returns the unicode codepoint ID (e.g. "1f408").
func (n *NotoEmojiIcon) ID() string {
	return n.entry.ID
}

// Title returns the emoji description / title.
func (n *NotoEmojiIcon) Title() string {
	return n.entry.Title
}

// Provider returns "noto-emoji".
func (n *NotoEmojiIcon) Provider() string {
	return "noto-emoji"
}

// Attribution returns "Google Noto Emoji".
func (n *NotoEmojiIcon) Attribution() string {
	return "Google Noto Emoji"
}

// Tags returns the tags/keywords associated with the emoji.
func (n *NotoEmojiIcon) Tags() []string {
	return n.entry.Tags
}

// Bytes retrieves the embedded 32x32 PNG data from the binary.
func (n *NotoEmojiIcon) Bytes() ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.data) > 0 {
		return n.data, nil
	}

	imgPath := path.Join("images", n.entry.Filename)
	data, err := notoicons.FS.ReadFile(imgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded noto emoji %s: %w", imgPath, err)
	}

	n.data = data
	hash := sha256.Sum256(data)
	n.sha256 = hex.EncodeToString(hash[:])

	return n.data, nil
}

// SHA256 returns the SHA-256 hex string of the icon's image bytes.
func (n *NotoEmojiIcon) SHA256() (string, error) {
	if n.sha256 != "" {
		return n.sha256, nil
	}

	data, err := n.Bytes()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	n.sha256 = hex.EncodeToString(hash[:])
	return n.sha256, nil
}

// NotoEmojiSearcher searches for icons within embedded Google Noto Emoji assets.
type NotoEmojiSearcher struct{}

func NewNotoEmojiSearcher() *NotoEmojiSearcher {
	return &NotoEmojiSearcher{}
}

// SearchForIcon finds icons matching any one of the given keywords using the precomputed tag index.
func (s *NotoEmojiSearcher) SearchForIcon(keywords []string) ([]Icon, error) {
	seenIndices := make(map[int]bool)
	var matches []Icon

	for _, kw := range keywords {
		cleaned := strings.ToLower(strings.TrimSpace(kw))
		if cleaned == "" {
			continue
		}

		if indices, found := notoicons.TagIndex[cleaned]; found {
			for _, idx := range indices {
				if !seenIndices[idx] && idx >= 0 && idx < len(notoicons.AllEmojis) {
					seenIndices[idx] = true
					matches = append(matches, NewNotoEmojiIcon(notoicons.AllEmojis[idx]))
				}
			}
		}
	}

	return matches, nil
}
