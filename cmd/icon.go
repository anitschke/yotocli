package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/vgaro/yotocli/internal/actions"
)

var iconCmd = &cobra.Command{
	Use:   "icon",
	Short: "Manage icons",
}

var uploadIconCmd = &cobra.Command{
	Use:   "upload <file_or_url>",
	Short: "Upload a custom icon (local file or URL)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		slog.Info("Uploading icon", "source", source)

		id, err := actions.UploadIcon(apiClient, source)
		if err != nil {
			return err
		}

		slog.Info("Icon uploaded successfully", "id", id)
		fmt.Println(id)
		return nil
	},
}

func init() {
	iconCmd.AddCommand(uploadIconCmd)
	rootCmd.AddCommand(iconCmd)
}
