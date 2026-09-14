package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vgaro/yotocli/internal/config"
	"github.com/vgaro/yotocli/pkg/yoto"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
	apiClient *yoto.Client
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yoto",
	Short: "A CLI tool for managing Yoto cards and players",
	Long: `YotoCLI is a tool for advanced users to manage their Yoto library.
It allows for uploading files, creating playlists, and managing device state directly from the terminal.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		initLogger()

		// Initialize the API client with the token from config
		token := config.GetAccessToken()
		clientID := config.GetClientID()
		apiClient = yoto.NewClient(token, clientID)

		// Check if token is valid by making a lightweight call
		// If unauthorized, try to refresh
		if token != "" {
			_, err := apiClient.ListDevices()
			if err != nil && (strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "401")) {
				slog.Info("Access token expired, attempting refresh")
				refreshToken := config.GetRefreshToken()
				if refreshToken == "" {
					return fmt.Errorf("authentication expired and no refresh token found. Please run 'yoto login'")
				}

				newTokens, refreshErr := apiClient.RefreshToken(refreshToken)
				if refreshErr != nil {
					return fmt.Errorf("failed to refresh token: %v. Please run 'yoto login'", refreshErr)
				}

				// Refresh tokens rotate and are single-use, so the new one must
				// replace the stored copy. If the response omitted one, keep the
				// existing token rather than wiping it.
				rotated := newTokens.RefreshToken
				if rotated == "" {
					rotated = refreshToken
				}
				config.SetToken(newTokens.AccessToken, rotated)
				if err := config.Save(); err != nil {
					return fmt.Errorf("failed to save new tokens: %w", err)
				}

				// Re-init client with new token
				apiClient = yoto.NewClient(newTokens.AccessToken, clientID)
				slog.Info("Token successfully refreshed")
			}
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	defer func() {
		if apiClient != nil {
			if err := apiClient.Close(); err != nil {
				slog.Warn("Failed to close client", "error", err)
			}
		}
	}()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// RootCmd returns the root command for doc generation
func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	cobra.OnInitialize(initConfig, initLogger)

	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/yotocli/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text, json)")

	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("log-format", rootCmd.PersistentFlags().Lookup("log-format"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in ~/.config/yotocli directory
		configPath := filepath.Join(home, ".config", "yotocli")
		viper.AddConfigPath(configPath)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.BindEnv("log-level", "YOTO_LOG_LEVEL", "LOG_LEVEL")
	viper.BindEnv("log-format", "YOTO_LOG_FORMAT", "LOG_FORMAT")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		// fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func initLogger() {
	lvlStr := viper.GetString("log-level")
	if lvlStr == "" {
		lvlStr = logLevel
	}
	if lvlStr == "" {
		lvlStr = "warn"
	}

	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(lvlStr)) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelWarn
	}

	fmtStr := viper.GetString("log-format")
	if fmtStr == "" {
		fmtStr = logFormat
	}
	if fmtStr == "" {
		fmtStr = "text"
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(strings.TrimSpace(fmtStr)) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}
