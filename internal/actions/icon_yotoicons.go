package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	YotoIconsBaseURL = "https://yotoicons.com"
	UserAgent        = "yotocli/1.0 (+https://github.com)"
)

// YotoIconsDotComIcon implements Icon for icons found on yotoicons.com.
type YotoIconsDotComIcon struct {
	id       string
	title    string
	tags     []string
	author   string
	imageURL string
	client   *http.Client

	mu     sync.Mutex
	data   []byte
	sha256 string
}

func NewYotoIconsDotComIcon(id, title string, tags []string, author, imageURL string, client *http.Client) *YotoIconsDotComIcon {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &YotoIconsDotComIcon{
		id:       id,
		title:    title,
		tags:     tags,
		author:   author,
		imageURL: imageURL,
		client:   client,
	}
}

// ID returns the unique identifier on yotoicons.com.
func (y *YotoIconsDotComIcon) ID() string {
	return y.id
}

// Title returns the title of the icon.
func (y *YotoIconsDotComIcon) Title() string {
	return y.title
}

// Provider returns "yotoicons.com".
func (y *YotoIconsDotComIcon) Provider() string {
	return "yotoicons.com"
}

// Attribution returns the author of the icon.
func (y *YotoIconsDotComIcon) Attribution() string {
	return y.author
}

// Tags returns the tags/categories associated with the icon.
func (y *YotoIconsDotComIcon) Tags() []string {
	return y.tags
}

// ImageURL returns the source URL on yotoicons.com.
func (y *YotoIconsDotComIcon) ImageURL() string {
	return y.imageURL
}

// Bytes downloads and returns the raw icon image bytes.
func (y *YotoIconsDotComIcon) Bytes() ([]byte, error) {
	y.mu.Lock()
	defer y.mu.Unlock()

	if len(y.data) > 0 {
		return y.data, nil
	}

	if y.imageURL == "" {
		return nil, fmt.Errorf("icon has no download url")
	}

	slog.Debug("Downloading icon bytes from yotoicons.com", "id", y.id, "url", y.imageURL)
	req, err := http.NewRequest(http.MethodGet, y.imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", y.imageURL, err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "image/png,image/*")

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch icon image from %s: %w", y.imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching icon from %s", resp.StatusCode, y.imageURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read icon body from %s: %w", y.imageURL, err)
	}

	y.data = data
	hash := sha256.Sum256(data)
	y.sha256 = hex.EncodeToString(hash[:])

	return y.data, nil
}

// SHA256 returns the SHA-256 hex string of the icon's image bytes.
func (y *YotoIconsDotComIcon) SHA256() (string, error) {
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

// YotoIconsDotComSearcher searches for icons on yotoicons.com.
type YotoIconsDotComSearcher struct {
	baseURL string
	client  *http.Client
}

func NewYotoIconsDotComSearcher(client *http.Client) *YotoIconsDotComSearcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &YotoIconsDotComSearcher{
		baseURL: YotoIconsBaseURL,
		client:  client,
	}
}

// SetBaseURL overrides the base URL (useful for testing).
func (s *YotoIconsDotComSearcher) SetBaseURL(baseURL string) {
	s.baseURL = baseURL
}

// SearchForIcon finds icons matching any one of the given keywords on yotoicons.com.
func (s *YotoIconsDotComSearcher) SearchForIcon(keywords []string) ([]Icon, error) {
	var cleanedKeywords []string
	for _, kw := range keywords {
		k := strings.ToLower(strings.TrimSpace(kw))
		if k != "" {
			cleanedKeywords = append(cleanedKeywords, k)
		}
	}

	if len(cleanedKeywords) == 0 {
		return nil, nil
	}

	slog.Info("Searching icons on yotoicons.com", "keywords", cleanedKeywords)

	seenIDs := make(map[string]bool)
	var allResults []Icon

	for _, kw := range cleanedKeywords {
		searchURL := fmt.Sprintf("%s/icons?tag=%s&type=singles&sort=popular", s.baseURL, url.QueryEscape(kw))
		req, err := http.NewRequest(http.MethodGet, searchURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create search request: %w", err)
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Accept", "text/html")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute search request on yotoicons.com: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("yotoicons.com returned status %d", resp.StatusCode)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read search response from yotoicons.com: %w", err)
		}

		items := parseYotoiconsHTML(string(bodyBytes), s.baseURL, s.client)
		for _, item := range items {
			if !seenIDs[item.ID()] {
				seenIDs[item.ID()] = true
				allResults = append(allResults, item)
			}
		}
	}

	return allResults, nil
}

// Regex to extract populate_icon_modal calls from yotoicons.com browse HTML:
// populate_icon_modal('id', 'category', 'tag1', 'tag2', 'author', 'downloads')
var yotoiconsModalRegex = regexp.MustCompile(`populate_icon_modal\(\s*'(\d+)'\s*,\s*'([^']*)'\s*,\s*'([^']*)'\s*,\s*'([^']*)'\s*,\s*'([^']*)'\s*,\s*'(\d+)'\s*\)`)

func parseYotoiconsHTML(html, baseURL string, client *http.Client) []*YotoIconsDotComIcon {
	matches := yotoiconsModalRegex.FindAllStringSubmatch(html, -1)
	var icons []*YotoIconsDotComIcon
	seen := make(map[string]bool)

	for _, match := range matches {
		id := match[1]
		if seen[id] {
			continue
		}
		seen[id] = true

		category := match[2]
		tag1 := match[3]
		tag2 := match[4]
		author := match[5]

		var tags []string
		for _, t := range []string{category, tag1, tag2, author} {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}

		var titleParts []string
		for _, t := range []string{tag1, tag2} {
			t = strings.TrimSpace(t)
			if t != "" {
				titleParts = append(titleParts, t)
			}
		}

		title := strings.Join(titleParts, " · ")
		if title == "" {
			title = category
		}
		if title == "" {
			title = fmt.Sprintf("Icon %s", id)
		}

		imageURL := fmt.Sprintf("%s/static/uploads/%s.png", baseURL, id)
		icon := NewYotoIconsDotComIcon(id, title, tags, author, imageURL, client)
		icons = append(icons, icon)
	}

	return icons
}
