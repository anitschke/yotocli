package actions

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestYotoIconsDotComSearcher(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	htmlResponse := `
	<html>
	<body>
		<div class="icon" onclick="populate_icon_modal('12583', 'animals', 'Grannies Bingo', 'Bluey Book Reads', 'curiouscat', '10051');">
			<div class="icon_background"><img src="/static/uploads/12583.png"></div>
		</div>
		<div class="icon" onclick="populate_icon_modal('8703', 'animals', 'gruffalo', 'Julia Donaldson', 'curiouscat', '5302');">
			<div class="icon_background"><img src="/static/uploads/8703.png"></div>
		</div>
	</body>
	</html>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/icons":
			tag := r.URL.Query().Get("tag")
			if tag == "cat" || tag == "bluey" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(htmlResponse))
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<html><body></body></html>`))
		case "/static/uploads/12583.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBuf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	searcher := NewYotoIconsDotComSearcher(server.Client())
	searcher.SetBaseURL(server.URL)

	icons, err := searcher.SearchForIcon([]string{"cat"})
	if err != nil {
		t.Fatalf("SearchForIcon failed: %v", err)
	}

	if len(icons) != 2 {
		t.Fatalf("expected 2 icons, got %d", len(icons))
	}

	icon0 := icons[0]
	if icon0.ID() != "12583" {
		t.Errorf("expected ID '12583', got %s", icon0.ID())
	}
	if icon0.Provider() != "yotoicons.com" {
		t.Errorf("expected Provider 'yotoicons.com', got %s", icon0.Provider())
	}
	if icon0.Title() != "Grannies Bingo · Bluey Book Reads" {
		t.Errorf("expected Title 'Grannies Bingo · Bluey Book Reads', got %s", icon0.Title())
	}
	if icon0.Attribution() != "curiouscat" {
		t.Errorf("expected Attribution 'curiouscat', got %s", icon0.Attribution())
	}

	// Test Bytes() fetching
	bytesData, err := icon0.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}
	if len(bytesData) == 0 {
		t.Errorf("expected non-empty bytes")
	}

	// Test SHA256()
	sha, err := icon0.SHA256()
	if err != nil {
		t.Fatalf("SHA256() failed: %v", err)
	}
	if sha == "" {
		t.Errorf("expected non-empty SHA256")
	}

	// Test caching in IconCache with yotoicons.com
	tmpDir, err := os.MkdirTemp("", "yotoicons-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cachedBytes, err := cache.Get(icon0)
	if err != nil {
		t.Fatalf("cache.Get failed: %v", err)
	}
	if len(cachedBytes) != len(bytesData) {
		t.Errorf("cached bytes mismatch")
	}

	foundID, provider, found := cache.FindBySHA256(sha)
	if !found || foundID != "12583" || provider != "yotoicons.com" {
		t.Errorf("cache FindBySHA256 expected (12583, yotoicons.com, true), got (%s, %s, %v)", foundID, provider, found)
	}
}
