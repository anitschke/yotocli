package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/vgaro/yotocli/pkg/yoto"
)

// YotoIcon implements the Icon interface for official Yoto icons.
type YotoIcon struct {
	client *yoto.Client
	icon   yoto.DisplayIcon

	mu     sync.Mutex
	data   []byte
	sha256 string
}

func NewYotoIcon(client *yoto.Client, icon yoto.DisplayIcon) *YotoIcon {
	return &YotoIcon{
		client: client,
		icon:   icon,
	}
}

// ID returns the icon ID used in cards (media ID, or displayIconId as fallback).
func (y *YotoIcon) ID() string {
	if y.icon.MediaID != "" {
		return y.icon.MediaID
	}
	return y.icon.DisplayIconID
}

// DisplayIconID returns the unique displayIconId.
func (y *YotoIcon) DisplayIconID() string {
	return y.icon.DisplayIconID
}

// Title returns the title of the icon.
func (y *YotoIcon) Title() string {
	return y.icon.Title
}

// Provider returns the provider name.
func (y *YotoIcon) Provider() string {
	return "yoto"
}

// Attribution returns attribution or author info.
func (y *YotoIcon) Attribution() string {
	return y.icon.UserId
}

// Tags returns public tags for this icon.
func (y *YotoIcon) Tags() []string {
	return y.icon.PublicTags
}

// URL returns the icon media URL.
func (y *YotoIcon) URL() string {
	return y.icon.URL
}

// Bytes loads the raw PNG data directly from the icon URL if not already in memory.
func (y *YotoIcon) Bytes() ([]byte, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if len(y.data) > 0 {
		return y.data, nil
	}

	if y.icon.URL == "" {
		return nil, fmt.Errorf("icon has no download url")
	}

	slog.Debug("Downloading icon bytes from provider", "id", y.ID(), "url", y.icon.URL)
	if y.client == nil {
		return nil, fmt.Errorf("no client available to fetch icon")
	}

	data, err := y.client.FetchBytes(y.icon.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch icon bytes from %s: %w", y.icon.URL, err)
	}

	y.data = data
	hash := sha256.Sum256(data)
	y.sha256 = hex.EncodeToString(hash[:])

	return y.data, nil
}

// SHA256 returns the SHA-256 hex string of the icon's image bytes.
func (y *YotoIcon) SHA256() (string, error) {
	if y.sha256 != "" {
		return y.sha256, nil
	}

	data, err := y.Bytes()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	y.sha256 = hex.EncodeToString(hash[:])
	return y.sha256, nil
}

// YotoIconSearcher implements IconSearcher by querying Yoto's public icons.
type YotoIconSearcher struct {
	client *yoto.Client
}

func NewYotoIconSearcher(client *yoto.Client) *YotoIconSearcher {
	return &YotoIconSearcher{
		client: client,
	}
}

// SearchForIcon searches through Yoto's public icons by matching any one of the given keywords in title or tags.
func (s *YotoIconSearcher) SearchForIcon(keywords []string) ([]Icon, error) {
	slog.Info("Fetching public icons from Yoto")
	publicIcons, err := s.client.GetPublicIcons()
	if err != nil {
		return nil, fmt.Errorf("failed to list public icons: %w", err)
	}

	var cleanedKeywords []string
	for _, kw := range keywords {
		k := strings.ToLower(strings.TrimSpace(kw))
		if k != "" {
			cleanedKeywords = append(cleanedKeywords, k)
		}
	}

	var matches []Icon
	for _, item := range publicIcons {
		if matchIcon(item, cleanedKeywords) {
			matches = append(matches, NewYotoIcon(s.client, item))
		}
	}

	return matches, nil
}

func matchIcon(item yoto.DisplayIcon, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}

	for _, kw := range keywords {
		// Check tags with exact match
		for _, tag := range item.PublicTags {
			if strings.EqualFold(strings.TrimSpace(tag), kw) {
				return true
			}
		}

		// Check title with word boundary match (so "cat" matches "Cat, animal" or "Keytar Cat", but not "Caterpillar" or "Dreamcatcher")
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		if err == nil && re.MatchString(item.Title) {
			return true
		}

		// Check mediaId or displayIconId exact match
		if strings.EqualFold(item.MediaID, kw) || strings.EqualFold(item.DisplayIconID, kw) {
			return true
		}
	}

	return false
}
