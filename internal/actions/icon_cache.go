package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// IconCache manages disk-cached icons organized by provider and an in-memory SHA256 index.
type IconCache struct {
	baseDir string

	mu      sync.RWMutex
	shaMap  map[string]cachedRef // sha256 hex -> provider/id pair
}

type cachedRef struct {
	provider string
	id       string
}

var (
	defaultCache     *IconCache
	defaultCacheOnce sync.Once
)

// GetDefaultIconCache returns the shared IconCache instance, rooted at standard cache location (~/.cache/yotocli/icons).
func GetDefaultIconCache() (*IconCache, error) {
	var initErr error
	defaultCacheOnce.Do(func() {
		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			initErr = fmt.Errorf("could not determine user cache directory: %w", err)
			return
		}
		baseDir := filepath.Join(cacheRoot, "yotocli", "icons")
		defaultCache, initErr = NewIconCache(baseDir)
	})
	return defaultCache, initErr
}

// NewIconCache creates an IconCache with the specified root directory and indexes existing files across all providers.
func NewIconCache(baseDir string) (*IconCache, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create icon cache directory %s: %w", baseDir, err)
	}

	c := &IconCache{
		baseDir: baseDir,
		shaMap:  make(map[string]cachedRef),
	}

	// Index existing cached icons across all provider subdirectories
	providerEntries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache dir %s: %w", baseDir, err)
	}

	for _, pEntry := range providerEntries {
		if !pEntry.IsDir() {
			continue
		}
		provider := pEntry.Name()
		providerDir := filepath.Join(baseDir, provider)
		iconEntries, err := os.ReadDir(providerDir)
		if err != nil {
			continue
		}

		for _, iEntry := range iconEntries {
			if iEntry.IsDir() {
				continue
			}
			filePath := filepath.Join(providerDir, iEntry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				slog.Debug("Failed to read cached icon file during indexing", "file", filePath, "error", err)
				continue
			}

			hash := sha256.Sum256(data)
			shaHex := hex.EncodeToString(hash[:])
			c.shaMap[shaHex] = cachedRef{provider: provider, id: iEntry.Name()}
		}
	}

	return c, nil
}

// Get retrieves the bytes for an icon. It checks if the icon is already in the cache
// for that provider and ID. If not present, it calls the icon's Bytes() method to obtain
// the data, stores it in the provider's cache directory, and records the SHA-256 in memory.
func (c *IconCache) Get(icon Icon) ([]byte, error) {
	if icon == nil {
		return nil, fmt.Errorf("icon cannot be nil")
	}

	provider := icon.Provider()
	id := icon.ID()
	if provider == "" || id == "" {
		return nil, fmt.Errorf("icon must have valid Provider and ID")
	}

	filePath := filepath.Join(c.baseDir, provider, id)

	c.mu.RLock()
	data, err := os.ReadFile(filePath)
	c.mu.RUnlock()

	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read cached icon file %s: %w", filePath, err)
	}

	// Not in cache, fetch using Icon.Bytes()
	data, err = icon.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get icon bytes from provider: %w", err)
	}

	// Put into cache
	if putErr := c.put(provider, id, data); putErr != nil {
		slog.Warn("Failed to cache icon bytes", "provider", provider, "id", id, "error", putErr)
	}

	return data, nil
}

// put writes an icon to its provider directory and updates the SHA256 map.
func (c *IconCache) put(provider, id string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	providerDir := filepath.Join(c.baseDir, provider)
	if err := os.MkdirAll(providerDir, 0755); err != nil {
		return fmt.Errorf("failed to create provider cache directory %s: %w", providerDir, err)
	}

	filePath := filepath.Join(providerDir, id)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cached icon %s/%s: %w", provider, id, err)
	}

	hash := sha256.Sum256(data)
	shaHex := hex.EncodeToString(hash[:])
	c.shaMap[shaHex] = cachedRef{provider: provider, id: id}

	return nil
}

// FindBySHA256 returns the icon ID and provider matching the given SHA256 hex string, if known.
func (c *IconCache) FindBySHA256(shaHex string) (string, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ref, ok := c.shaMap[shaHex]
	return ref.id, ref.provider, ok
}

// StoreCached registers an icon's data in the cache for a given provider and ID.
func (c *IconCache) StoreCached(provider, id string, data []byte) error {
	return c.put(provider, id, data)
}
