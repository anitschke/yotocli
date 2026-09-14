package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/vgaro/yotocli/internal/actions"
	"github.com/vgaro/yotocli/pkg/yoto"
)

func TestIconSearchCommand(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/media/displayIcons/user/yoto":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"displayIcons": [
					{
						"displayIconId": "icon123",
						"mediaId": "media123",
						"title": "Sunflower",
						"url": "http://%s/sunflower.png",
						"userId": "yoto",
						"public": true,
						"new": true,
						"publicTags": ["flower", "plant"]
					}
				]
			}`, r.Host)
		case "/sunflower.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBuf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	apiClient = yoto.NewClient("fake-token", "fake-client")
	apiClient.SetBaseURL(server.URL)

	tmpDir, err := os.MkdirTemp("", "icon-cmd-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := actions.NewIconCache(tmpDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	searcher := actions.NewYotoIconSearcher(apiClient)
	icons, err := searcher.SearchForIcon([]string{"flower"})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon, got %d", len(icons))
	}

	var outBuf bytes.Buffer
	// Temporarily capture stdout or test helper directly
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printIconResult(icons[0], cache)

	w.Close()
	os.Stdout = oldStdout
	outBuf.ReadFrom(r)

	output := outBuf.String()
	if !strings.Contains(output, "ID: media123") {
		t.Errorf("expected output to contain ID: media123, got:\n%s", output)
	}
	if !strings.Contains(output, "Title: Sunflower") {
		t.Errorf("expected output to contain Title: Sunflower, got:\n%s", output)
	}
	if !strings.Contains(output, "Tags: flower, plant") {
		t.Errorf("expected output to contain Tags: flower, plant, got:\n%s", output)
	}
	// Check terminal half block character
	if !strings.Contains(output, "▀") {
		t.Errorf("expected output to contain half-block character ▀, got:\n%s", output)
	}
}
