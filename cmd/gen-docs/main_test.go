package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func TestDocsUpToDate(t *testing.T) {
	tempDir := t.TempDir()
	if err := GenerateDocs(tempDir); err != nil {
		t.Fatalf("failed to generate docs into temp directory: %v", err)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("failed to find repository root: %v", err)
	}

	actualDir := filepath.Join(repoRoot, "docs", "commands")
	actualEntries, err := os.ReadDir(actualDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", actualDir, err)
	}

	tempEntries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", tempDir, err)
	}

	actualFiles := make(map[string]bool)
	for _, entry := range actualEntries {
		if !entry.IsDir() {
			actualFiles[entry.Name()] = true
		}
	}

	tempFiles := make(map[string]bool)
	for _, entry := range tempEntries {
		if !entry.IsDir() {
			tempFiles[entry.Name()] = true
		}
	}

	// Check for missing files
	for filename := range tempFiles {
		if !actualFiles[filename] {
			t.Errorf("missing doc file: docs/commands/%s; run 'make docs' to regenerate", filename)
		}
	}

	// Check for stale files that should no longer exist
	for filename := range actualFiles {
		if !tempFiles[filename] {
			t.Errorf("stale doc file: docs/commands/%s; run 'make docs' to regenerate", filename)
		}
	}

	// Check file contents
	for filename := range tempFiles {
		if !actualFiles[filename] {
			continue
		}

		expectedContent, err := os.ReadFile(filepath.Join(tempDir, filename))
		if err != nil {
			t.Fatalf("failed to read generated %s: %v", filename, err)
		}

		actualContent, err := os.ReadFile(filepath.Join(actualDir, filename))
		if err != nil {
			t.Fatalf("failed to read existing %s: %v", filename, err)
		}

		if !bytes.Equal(expectedContent, actualContent) {
			t.Errorf("docs/commands/%s is out of date; run 'make docs' to regenerate", filename)
		}
	}
}
