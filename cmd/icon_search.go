package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vgaro/yotocli/internal/actions"
)

var searchProvider string

var searchIconCmd = &cobra.Command{
	Use:   "search <keyword> [keywords...]",
	Short: "Search for icons by title or tag",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slog.Info("Searching for icons", "keywords", args, "provider", searchProvider)

		cache, err := actions.GetDefaultIconCache()
		if err != nil {
			slog.Warn("Failed to initialize icon cache", "error", err)
		}

		var searchers []actions.IconSearcher
		switch strings.ToLower(strings.TrimSpace(searchProvider)) {
		case "yoto":
			searchers = append(searchers, actions.NewYotoIconSearcher(apiClient))
		case "yotoicons", "yotoicons.com":
			searchers = append(searchers, actions.NewYotoIconsDotComSearcher(nil))
		case "all", "":
			searchers = append(searchers, actions.NewYotoIconSearcher(apiClient))
			searchers = append(searchers, actions.NewYotoIconsDotComSearcher(nil))
		default:
			return fmt.Errorf("unknown provider: %q (supported: all, yoto, yotoicons.com)", searchProvider)
		}

		var allIcons []actions.Icon
		for _, s := range searchers {
			icons, err := s.SearchForIcon(args)
			if err != nil {
				return err
			}
			allIcons = append(allIcons, icons...)
		}

		if len(allIcons) == 0 {
			slog.Info("No icons found matching query", "keywords", args)
			return nil
		}

		for _, icon := range allIcons {
			printIconResult(icon, cache)
		}

		return nil
	},
}

func printIconResult(icon actions.Icon, cache *actions.IconCache) {
	fmt.Fprintf(os.Stdout, "ID: %s\n", icon.ID())
	fmt.Fprintf(os.Stdout, "Title: %s\n", icon.Title())
	fmt.Fprintf(os.Stdout, "Provider: %s\n", icon.Provider())
	if attr := icon.Attribution(); attr != "" {
		fmt.Fprintf(os.Stdout, "Attribution: %s\n", attr)
	}

	if tagger, ok := icon.(actions.Tagger); ok {
		if tags := tagger.Tags(); len(tags) > 0 {
			fmt.Fprintf(os.Stdout, "Tags: %s\n", strings.Join(tags, ", "))
		}
	}

	// Fetch bytes (via cache if available) and render terminal preview
	var data []byte
	var err error
	if cache != nil {
		data, err = cache.Get(icon)
	} else {
		data, err = icon.Bytes()
	}
	if err != nil {
		slog.Debug("Could not fetch icon bytes for preview", "id", icon.ID(), "error", err)
	} else {
		lines, err := actions.RenderIconHalfBlocks(data)
		if err != nil {
			slog.Debug("Could not render icon preview", "id", icon.ID(), "error", err)
		} else {
			for _, line := range lines {
				fmt.Fprintln(os.Stdout, line)
			}
		}
	}
	fmt.Fprintln(os.Stdout)
}

func init() {
	searchIconCmd.Flags().StringVarP(&searchProvider, "provider", "p", "all", "Icon provider to search: all, yoto, yotoicons.com")
	iconCmd.AddCommand(searchIconCmd)
}
