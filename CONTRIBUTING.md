# Contributing to YotoCLI

Thank you for helping improve YotoCLI!

> [!NOTE]
> Comprehensive guidelines for development, architecture, coding standards, and `log/slog` logging conventions are documented in [AGENTS.md](file:///home/anitschk/sandbox/yotocli/AGENTS.md), which serves as the unified reference for human contributors and AI agents.

## Quickstart & Development Setup

1. **Go:** Ensure you have Go 1.24+ installed.
2. **External Tools:** `yt-dlp` and `ffmpeg` are only needed if working on `yoto import`. Audio is normalized by Yoto servers during transcoding.
3. **Dependencies:** Run `go mod download`.

## Running Tests

Run the full test suite:
```bash
go test ./...
```

Run specific packages:
```bash
go test -v ./cmd/...
go test -v ./internal/utils/...
go test -v ./pkg/yoto/...
```

## Adding Commands

1. Create a new command file in `cmd/`.
2. Register your command on `rootCmd`.
3. Interact with Yoto using `apiClient` (available in the `cmd` package) or actions in `internal/actions`.
4. Include an `Example` section in your Cobra command definition.

## Core Rules

- **Logging & Unix Philosophy:** Never print progress or status updates using `fmt.Print*` to stdout. Use `log/slog` (`slog.Info`, `slog.Debug`, `slog.Warn`). Only write explicit, user-requested output to stdout. See [AGENTS.md](file:///home/anitschk/sandbox/yotocli/AGENTS.md) for log level conventions.
- **Layering:** Keep CLI/flag presentation in `cmd/`, reusable API logic in `pkg/yoto/`, and multi-step workflows in `internal/actions/`.
- **Error Handling:** Return errors from packages and let Cobra command `RunE` propagate them.
- **Sanitization:** Always use `utils.SanitizeFilename` when creating local files from API data.
- **Metadata Preservation:** Always preserve unmodeled fields using `pkg/yoto/passthrough.go` when updating cards.
