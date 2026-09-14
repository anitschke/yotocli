# Contributor & Agent Guidelines (AGENTS.md)

Welcome to `yotocli`! This document serves as the central guide for human contributors and autonomous AI agents working in this codebase. It outlines architecture conventions, coding standards, logging practices with `log/slog`, and development workflows.

---

## 1. Project Overview & Philosophy

`yotocli` is a native command-line tool for managing Yoto cards, playlists, and player devices.

### Core Principles
1. **Unix Philosophy ("Silence is Golden")**:
   - Commands should only produce output on `os.Stdout` if the user explicitly requested data (e.g., `yoto ls`, `yoto status`, or `yoto icon upload` printing the resulting icon ID).
   - Informational status, upload/download progress, and operational diagnostics must **never** be printed using `fmt.Print*` to `os.Stdout`. Instead, log them via `log/slog`.
   - By default, operations run quietly.
2. **Filesystem Abstraction**:
   - The Yoto library is treated conceptually as a filesystem:
     - **Card / Playlist** $\rightarrow$ Directory
     - **Track / Chapter** $\rightarrow$ File
     - Commands like `ls`, `mv`, `cp`, `rm` directly reflect filesystem-style management using slash syntax (e.g., `Playlist/Track`).
3. **Strict Layering & Non-CLI Reusability**:
   - `pkg/yoto/` is a self-contained client library with zero CLI or Cobra dependencies.
   - `cmd/` handles CLI arguments, flags, and presents final outputs.
   - `internal/actions/` coordinates high-level business flows between `cmd/` and `pkg/`.

---

## 2. Logging Architecture with `log/slog`

The project utilizes Go's standard library structured logging package (`log/slog`).

### Global Default Logger Pattern
We do **not** thread `*slog.Logger` or custom logging callbacks through function signatures. Instead:
- Cobra initializes the logger during `rootCmd` startup and calls `slog.SetDefault(logger)`.
- Any package or file across the project simply imports `"log/slog"` and calls package-level functions: `slog.Debug(...)`, `slog.Info(...)`, `slog.Warn(...)`, `slog.Error(...)`.

### Output Stream & Format
- **Destination**: All logs are directed to `os.Stderr`. This guarantees `os.Stdout` remains clean for pipes and automation.
- **Format**: Text format (`slog.NewTextHandler`) is default for terminal readability. JSON format (`slog.NewJSONHandler`) is supported via `--log-format json`.

### Log Level Guidelines
The CLI defaults to the **`warn`** log level (`slog.LevelWarn`).

| Level | When to Use | Examples |
|---|---|---|
| **DEBUG** | High-frequency or granular step-by-step progress | `[1/10] Downloading track...`, `Uploading track`, `Track already on Yoto, skipping` |
| **INFO** | Coarse-grained operational milestones | `Downloading playlist`, `Creating playlist`, `Playing card on device`, `Updating title` |
| **WARN** | Recoverable errors, deprecated parameters, or non-fatal partial failures | Device status query failure, ignored flags, failed to delete temp file |
| **ERROR** | Unrecoverable failures terminating execution | API token errors, network failure before exit |

### User Configuration (Cobra & Viper)
Logging is fully configurable via CLI flags, environment variables, or config file:
- **CLI Flags**:
  - `--log-level` (`debug`, `info`, `warn`, `error` — default: `warn`)
  - `--log-format` (`text`, `json` — default: `text`)
- **Environment Variables**:
  - `YOTO_LOG_LEVEL=debug`
  - `YOTO_LOG_FORMAT=json`
- **Config File (`~/.config/yotocli/config.yaml`)**:
  ```yaml
  log-level: info
  log-format: text
  ```

---

## 3. Codebase Structure

```
yotocli/
├── cmd/                # Cobra commands (CLI flags, stdout presentation, entrypoints)
├── pkg/
│   └── yoto/           # Standalone Yoto API client (HTTP requests, OAuth2, models)
├── internal/
│   ├── actions/        # High-level orchestration (AddTracks, ImportFromURL, etc.)
│   ├── config/         # Viper configuration manager (~/.config/yotocli/config.yaml)
│   ├── processing/     # Audio extraction and external downloader (yt-dlp)
│   └── utils/          # Filename sanitization, index parsing, track/card finders
├── docs/               # Architecture documents and notes
├── AGENTS.md           # Instructions for AI agents and human contributors
├── CONTRIBUTING.md     # Contribution guide (aligned with AGENTS.md)
├── go.mod              # Go module definition (Go 1.24+)
└── main.go             # Main application entrypoint
```

---

## 4. Key Implementation Rules for Agents & Contributors

1. **No Stdout Noise**:
   - Never use `fmt.Printf`, `fmt.Println`, or `println` for status updates or progress messages.
   - Use `slog.Info(...)` or `slog.Debug(...)`.
   - Reserve `fmt.Print*` exclusively for intended command outputs on stdout (e.g. `w := tabwriter.NewWriter(os.Stdout, ...)` or returning created entity IDs).

2. **Passthrough Preservation**:
   - The Yoto API returns fields not fully modeled in Go structs (e.g., custom cover art, ambient chapter lights, nightlight modes).
   - In `pkg/yoto/passthrough.go`, unknown fields are captured into raw maps and merged back when sending `PUT`/`PATCH` updates. Never strip or bypass this passthrough logic, as doing so will erase user metadata on Yoto cards.

3. **Audio Normalization**:
   - Do **not** run local ffmpeg normalization passes. Yoto's transcoder normalizes all uploads to -16 LUFS and transcodes to Opus on their ingestion pipeline.

4. **Filename Safety**:
   - Always sanitize titles when writing to the local filesystem using `utils.SanitizeFilename(...)`.

5. **Error Handling**:
   - Return clear, wrapped errors (`fmt.Errorf("...: %w", err)`) from `pkg/` and `internal/`.
   - Let Cobra command `RunE` return the error so that exit codes and error output are handled consistently at the root.

---

## 5. Development & Testing Workflow

### Prerequisites
- **Go 1.24+**
- **yt-dlp** and **ffmpeg** (only needed if developing/testing `yoto import`)

### Running Tests
Run the test suite across all packages:
```bash
go test ./...
```

Run tests for a specific package with verbose logging:
```bash
go test -v ./cmd/...
go test -v ./pkg/yoto/...
go test -v ./internal/...
```

### Build Binary
```bash
go build -o yoto main.go
./yoto --help
```
