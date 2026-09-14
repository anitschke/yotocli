package actions

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/vgaro/yotocli/pkg/yoto"
)

func TestYotoIconSearcher(t *testing.T) {
	pngBytes := createTestPNG(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/media/displayIcons/user/yoto":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"displayIcons": [
					{
						"displayIconId": "icon1",
						"mediaId": "media1",
						"title": "Seedling Plant",
						"url": "http://%s/icons/media1",
						"userId": "yoto",
						"public": true,
						"new": true,
						"publicTags": ["plant", "happy"]
					},
					{
						"displayIconId": "icon2",
						"mediaId": "media2",
						"title": "Rocket Ship",
						"url": "http://%s/icons/media2",
						"userId": "yoto",
						"public": true,
						"new": true,
						"publicTags": ["space", "vehicle"]
					}
				]
			}`, r.Host, r.Host)
		case "/icons/media1":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := yoto.NewClient("fake-token", "fake-client-id")
	client.SetBaseURL(server.URL)

	tmpDir, err := os.MkdirTemp("", "icon-search-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("failed to init cache: %v", err)
	}

	searcher := NewYotoIconSearcher(client)

	// Search for "plant"
	icons, err := searcher.SearchForIcon([]string{"plant"})
	if err != nil {
		t.Fatalf("SearchForIcon failed: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("Expected 1 icon for 'plant', got %d", len(icons))
	}
	if icons[0].Title() != "Seedling Plant" {
		t.Errorf("Expected 'Seedling Plant', got %s", icons[0].Title())
	}
	if icons[0].ID() != "media1" {
		t.Errorf("Expected 'media1', got %s", icons[0].ID())
	}

	// Search matching any keyword (OR): "nonexistent" OR "space" should match Rocket Ship
	multiIcons, err := searcher.SearchForIcon([]string{"nonexistent", "space"})
	if err != nil {
		t.Fatalf("SearchForIcon with multi keywords failed: %v", err)
	}
	if len(multiIcons) != 1 || multiIcons[0].Title() != "Rocket Ship" {
		t.Fatalf("Expected Rocket Ship matching 'space', got %v", multiIcons)
	}

	// Verify exact tag matching: keyword that only partially matches a tag (like "vehic" for "vehicle") should not match
	partialTagOnlyIcons, err := searcher.SearchForIcon([]string{"vehic"})
	if err != nil {
		t.Fatalf("SearchForIcon failed: %v", err)
	}
	if len(partialTagOnlyIcons) != 0 {
		t.Errorf("Expected 0 icons for partial tag 'vehic', got %d", len(partialTagOnlyIcons))
	}

	// Fetch Bytes through cache.Get(icon)
	data, err := cache.Get(icons[0])
	if err != nil {
		t.Fatalf("cache.Get failed: %v", err)
	}
	if !bytes.Equal(data, pngBytes) {
		t.Errorf("Bytes() did not match png data")
	}

	// Verify it was cached on disk in provider subfolder
	cachedData, err := os.ReadFile(tmpDir + "/yoto/media1")
	if err != nil || !bytes.Equal(cachedData, pngBytes) {
		t.Errorf("Icon media1 was not found in yoto cache folder: %v", err)
	}

	// Verify SHA256
	shaHex, err := icons[0].SHA256()
	if err != nil {
		t.Fatalf("SHA256() failed: %v", err)
	}
	if shaHex == "" {
		t.Errorf("Expected non-empty sha256")
	}

	// Find in cache by sha256
	id, provider, found := cache.FindBySHA256(shaHex)
	if !found || id != "media1" || provider != "yoto" {
		t.Errorf("Expected to find media1/yoto in cache by sha256, got id=%s, provider=%s, found=%v", id, provider, found)
	}
}
