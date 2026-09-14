package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vgaro/yotocli/pkg/yoto"
)

// UploadIcon uploads an icon from a local path or URL.
// It checks the local SHA-256 icon cache/index first; if an icon with the exact
// same image data has already been cached/indexed, it returns the existing Icon ID
// without re-uploading.
// Returns the Icon ID.
func UploadIcon(client *yoto.Client, source string) (string, error) {
	path := source
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		tmpDir := os.TempDir()
		path = filepath.Join(tmpDir, "yoto_icon_temp.png")
		if err := client.DownloadFile(source, path); err != nil {
			return "", err
		}
		defer os.Remove(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read icon file %s: %w", path, err)
	}

	hash := sha256.Sum256(data)
	shaHex := hex.EncodeToString(hash[:])

	cache, err := GetDefaultIconCache()
	if err == nil && cache != nil {
		if existingID, provider, found := cache.FindBySHA256(shaHex); found && provider == "yoto" {
			slog.Info("Icon with matching sha256 already exists on Yoto, skipping upload", "id", existingID, "sha256", shaHex)
			return existingID, nil
		}
	}

	slog.Info("Uploading icon to Yoto", "source", source)
	newID, err := client.UploadIcon(path)
	if err != nil {
		return "", err
	}

	if cache != nil {
		if err := cache.StoreCached("yoto", newID, data); err != nil {
			slog.Warn("Failed to cache uploaded icon", "id", newID, "error", err)
		}
	}

	return newID, nil
}
