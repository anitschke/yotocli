package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

type mockIcon struct {
	id       string
	provider string
	title    string
	data     []byte
}

func (m *mockIcon) ID() string                { return m.id }
func (m *mockIcon) Provider() string          { return m.provider }
func (m *mockIcon) Title() string             { return m.title }
func (m *mockIcon) Bytes() ([]byte, error)    { return m.data, nil }
func (m *mockIcon) SHA256() (string, error)   { return "", nil }
func (m *mockIcon) Attribution() string       { return "" }

func TestIconCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "icon-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("NewIconCache failed: %v", err)
	}

	testData := []byte("fake-icon-bytes")
	hash := sha256.Sum256(testData)
	shaHex := hex.EncodeToString(hash[:])

	// Initially not found by SHA
	if _, _, found := cache.FindBySHA256(shaHex); found {
		t.Errorf("Expected hash not found")
	}

	icon := &mockIcon{
		id:       "icon-1",
		provider: "yoto",
		title:    "Test Icon",
		data:     testData,
	}

	// First Get should fetch from icon and cache to disk
	gotBytes, err := cache.Get(icon)
	if err != nil {
		t.Fatalf("cache.Get failed: %v", err)
	}
	if string(gotBytes) != string(testData) {
		t.Errorf("Expected %q, got %q", string(testData), string(gotBytes))
	}

	// Verify file was written in provider subfolder
	providerFilePath := filepath.Join(tmpDir, "yoto", "icon-1")
	diskBytes, err := os.ReadFile(providerFilePath)
	if err != nil {
		t.Fatalf("expected file on disk at %s: %v", providerFilePath, err)
	}
	if string(diskBytes) != string(testData) {
		t.Errorf("expected disk data %q, got %q", string(testData), string(diskBytes))
	}

	// Subsequent Get should read from disk without calling icon.Bytes()
	iconEmptyData := &mockIcon{
		id:       "icon-1",
		provider: "yoto",
		title:    "Test Icon",
		data:     nil, // if it calls icon.Bytes(), it would return nil
	}
	gotBytes2, err := cache.Get(iconEmptyData)
	if err != nil {
		t.Fatalf("cache.Get on cached icon failed: %v", err)
	}
	if string(gotBytes2) != string(testData) {
		t.Errorf("Expected cached bytes from disk, got %q", string(gotBytes2))
	}

	// Find by SHA
	id, provider, found := cache.FindBySHA256(shaHex)
	if !found || id != "icon-1" || provider != "yoto" {
		t.Errorf("FindBySHA256 failed: found=%v, id=%s, provider=%s", found, id, provider)
	}

	// New cache instance on existing dir should pre-index files across provider dirs
	cache2, err := NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("NewIconCache 2 failed: %v", err)
	}
	id2, provider2, found2 := cache2.FindBySHA256(shaHex)
	if !found2 || id2 != "icon-1" || provider2 != "yoto" {
		t.Errorf("cache2 FindBySHA256 failed: found=%v, id=%s, provider=%s", found2, id2, provider2)
	}

	// Test that an unexpected read error (other than not exist) is returned
	unreadablePath := filepath.Join(tmpDir, "yoto", "unreadable-icon")
	if err := os.WriteFile(unreadablePath, []byte("hidden"), 0000); err == nil {
		unreadableIcon := &mockIcon{
			id:       "unreadable-icon",
			provider: "yoto",
			title:    "Unreadable",
			data:     []byte("hidden"),
		}
		_, err := cache.Get(unreadableIcon)
		// On non-root Linux, reading 0000 file returns permission denied error
		if os.Geteuid() != 0 {
			if err == nil {
				t.Errorf("expected error reading unreadable cached file, got nil")
			}
		}
		_ = os.Chmod(unreadablePath, 0644)
	}
}
