package actions

import (
	"os"
	"testing"
)

func TestNotoEmojiSearcher(t *testing.T) {
	searcher := NewNotoEmojiSearcher()
	icons, err := searcher.SearchForIcon([]string{"cat"})
	if err != nil {
		t.Fatalf("SearchForIcon failed: %v", err)
	}

	if len(icons) == 0 {
		t.Fatalf("expected to find icons for keyword 'cat'")
	}

	first := icons[0]
	if first.Provider() != "noto-emoji" {
		t.Errorf("expected provider 'noto-emoji', got %s", first.Provider())
	}
	if first.Attribution() != "Google Noto Emoji" {
		t.Errorf("expected attribution 'Google Noto Emoji', got %s", first.Attribution())
	}

	bytesData, err := first.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}
	if len(bytesData) == 0 {
		t.Errorf("expected non-empty bytes")
	}

	// Verify it implements Tagger
	tagger, ok := first.(Tagger)
	if !ok {
		t.Fatalf("expected NotoEmojiIcon to implement Tagger")
	}
	if len(tagger.Tags()) == 0 {
		t.Errorf("expected non-empty tags for cat emoji")
	}

	// Test cache interaction
	tmpDir, err := os.MkdirTemp("", "noto-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cachedBytes, err := cache.Get(first)
	if err != nil {
		t.Fatalf("cache.Get failed: %v", err)
	}
	if len(cachedBytes) != len(bytesData) {
		t.Errorf("cached bytes length mismatch")
	}

	sha, err := first.SHA256()
	if err != nil {
		t.Fatalf("SHA256 failed: %v", err)
	}
	foundID, provider, found := cache.FindBySHA256(sha)
	if !found || foundID != first.ID() || provider != "noto-emoji" {
		t.Errorf("cache FindBySHA256 failed: found=%v, id=%s, provider=%s", found, foundID, provider)
	}
}
