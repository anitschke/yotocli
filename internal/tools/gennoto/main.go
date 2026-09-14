package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

const (
	// Pinned release versions for build reproducibility
	notoEmojiVersion = "v2.051"
	cldrVersion      = "48.2.1"

	notoTarURL = "https://codeload.github.com/googlefonts/noto-emoji/tar.gz/" + notoEmojiVersion
	cldrURL    = "https://raw.githubusercontent.com/unicode-org/cldr-json/" + cldrVersion + "/cldr-json/cldr-annotations-full/annotations/en/annotations.json"
)

type CLDRAnnotations struct {
	Annotations struct {
		Annotations map[string]struct {
			Default []string `json:"default"`
			TTS     []string `json:"tts"`
		} `json:"annotations"`
	} `json:"annotations"`
}

type NotoEmojiEntry struct {
	ID       string   // e.g. "1f408" or "1f408_200d_2b1b"
	Filename string   // e.g. "emoji_u1f408.png"
	Title    string   // e.g. "cat"
	Tags     []string // e.g. ["animal", "cat", "feline"]
}

func main() {
	imagesDir := filepath.Join("internal", "actions", "notoicons", "images")
	outputGoFile := filepath.Join("internal", "actions", "notoicons", "generated_data.go")

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		log.Fatalf("failed to create images dir: %v", err)
	}

	// 1. Fetch CLDR annotations
	log.Println("Fetching Unicode CLDR annotations...")
	cldrMap, err := fetchCLDRAnnotations()
	if err != nil {
		log.Fatalf("failed to fetch CLDR annotations: %v", err)
	}
	log.Printf("Loaded %d CLDR annotation entries", len(cldrMap))

	// 2. Load or download png/32 images from noto-emoji
	downloadedFiles, err := getOrDownloadPNGs(imagesDir)
	if err != nil {
		log.Fatalf("failed to obtain pngs: %v", err)
	}
	log.Printf("Loaded %d PNG files", len(downloadedFiles))

	// 3. Match each downloaded PNG to CLDR annotations
	sort.Strings(downloadedFiles)
	var entries []NotoEmojiEntry
	tagIndex := make(map[string][]int) // tag -> slice of indices in entries

	wordRegex := regexp.MustCompile(`[\w]+`)

	for _, fname := range downloadedFiles {
		// fname is e.g. "emoji_u1f408.png" or "emoji_u1f408_200d_2b1b.png"
		id := strings.TrimPrefix(fname, "emoji_u")
		id = strings.TrimSuffix(id, ".png")

		chars := codepointsToRunes(id)
		annotation, found := cldrMap[chars]
		if !found {
			// Try with FE0F (variation selector-16) added if single/double codepoint
			charsWithVS := codepointsToRunesWithFE0F(id)
			annotation, found = cldrMap[charsWithVS]
		}

		title := ""
		var tags []string
		if found {
			if len(annotation.TTS) > 0 {
				title = annotation.TTS[0]
			}
			tags = annotation.Default
		}

		if title == "" {
			// Fallback title from ID
			title = fmt.Sprintf("Emoji %s", id)
		}

		idx := len(entries)
		entry := NotoEmojiEntry{
			ID:       id,
			Filename: fname,
			Title:    title,
			Tags:     tags,
		}
		entries = append(entries, entry)

		// Populate inverted index map
		seenInThisEntry := make(map[string]bool)

		// Index all tags
		for _, tag := range tags {
			t := strings.ToLower(strings.TrimSpace(tag))
			if t != "" && !seenInThisEntry[t] {
				seenInThisEntry[t] = true
				tagIndex[t] = append(tagIndex[t], idx)
			}
		}

		// Index words from title
		words := wordRegex.FindAllString(strings.ToLower(title), -1)
		for _, w := range words {
			if w != "" && !seenInThisEntry[w] {
				seenInThisEntry[w] = true
				tagIndex[w] = append(tagIndex[w], idx)
			}
		}

		// Also index the ID itself
		if !seenInThisEntry[id] {
			seenInThisEntry[id] = true
			tagIndex[id] = append(tagIndex[id], idx)
		}
	}

	// 4. Generate Go code
	log.Printf("Generating %s...", outputGoFile)
	if err := generateGoSource(outputGoFile, entries, tagIndex); err != nil {
		log.Fatalf("failed to generate Go source: %v", err)
	}

	log.Println("Done generating Noto emoji icons and index!")
}

func fetchCLDRAnnotations() (map[string]struct {
	Default []string
	TTS     []string
}, error) {
	resp, err := http.Get(cldrURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var data CLDRAnnotations
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	res := make(map[string]struct {
		Default []string
		TTS     []string
	})
	for k, v := range data.Annotations.Annotations {
		res[k] = struct {
			Default []string
			TTS     []string
		}{
			Default: v.Default,
			TTS:     v.TTS,
		}
	}
	return res, nil
}

func getOrDownloadPNGs(destDir string) ([]string, error) {
	entries, err := os.ReadDir(destDir)
	if err == nil {
		var existing []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "emoji_u") && strings.HasSuffix(e.Name(), ".png") {
				existing = append(existing, e.Name())
			}
		}
		if len(existing) > 1000 {
			log.Printf("Found %d existing PNGs in %s, skipping download", len(existing), destDir)
			return existing, nil
		}
	}

	log.Println("Downloading and extracting png/32 from googlefonts/noto-emoji...")
	return extractNoto32PNGs(destDir)
}

func extractNoto32PNGs(destDir string) ([]string, error) {
	resp, err := http.Get(notoTarURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d downloading noto tarball", resp.StatusCode)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var filenames []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Look for files inside */png/32/*.png
		if header.Typeflag == tar.TypeReg && strings.Contains(header.Name, "/png/32/") && strings.HasSuffix(header.Name, ".png") {
			base := filepath.Base(header.Name)
			outPath := filepath.Join(destDir, base)
			outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return nil, err
			}
			outFile.Close()
			filenames = append(filenames, base)
		}
	}

	return filenames, nil
}

func codepointsToRunes(hexStr string) string {
	parts := strings.Split(hexStr, "_")
	var runes []rune
	for _, p := range parts {
		if cp, err := strconv.ParseInt(p, 16, 32); err == nil {
			runes = append(runes, rune(cp))
		}
	}
	return string(runes)
}

func codepointsToRunesWithFE0F(hexStr string) string {
	parts := strings.Split(hexStr, "_")
	var runes []rune
	for _, p := range parts {
		if cp, err := strconv.ParseInt(p, 16, 32); err == nil {
			runes = append(runes, rune(cp))
		}
	}
	runes = append(runes, 0xFE0F)
	return string(runes)
}

type templateData struct {
	Entries  []NotoEmojiEntry
	TagIndex []tagIndexItem
}

type tagIndexItem struct {
	Tag     string
	Indices []int
}

const sourceTemplate = `// Code generated by internal/tools/gennoto. DO NOT EDIT.

package notoicons

type NotoEmojiEntry struct {
	ID       string
	Filename string
	Title    string
	Tags     []string
}

var AllEmojis = []NotoEmojiEntry{
{{- range .Entries }}
	{
		ID:       {{ printf "%q" .ID }},
		Filename: {{ printf "%q" .Filename }},
		Title:    {{ printf "%q" .Title }},
		Tags:     []string{ {{ range $i, $t := .Tags }}{{ if $i }}, {{ end }}{{ printf "%q" $t }}{{ end }} },
	},
{{- end }}
}

var TagIndex = map[string][]int{
{{- range .TagIndex }}
	{{ printf "%q" .Tag }}: { {{ range $i, $idx := .Indices }}{{ if $i }}, {{ end }}{{ $idx }}{{ end }} },
{{- end }}
}
`

var parsedTemplate = template.Must(template.New("notoicons").Parse(sourceTemplate))

func generateGoSource(destPath string, entries []NotoEmojiEntry, tagIndex map[string][]int) error {
	var sortedTags []string
	for k := range tagIndex {
		sortedTags = append(sortedTags, k)
	}
	sort.Strings(sortedTags)

	var indexItems []tagIndexItem
	for _, tag := range sortedTags {
		indexItems = append(indexItems, tagIndexItem{
			Tag:     tag,
			Indices: tagIndex[tag],
		})
	}

	data := templateData{
		Entries:  entries,
		TagIndex: indexItems,
	}

	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		_ = os.WriteFile(destPath, buf.Bytes(), 0644)
		return fmt.Errorf("gofmt error: %w", err)
	}

	return os.WriteFile(destPath, formatted, 0644)
}
