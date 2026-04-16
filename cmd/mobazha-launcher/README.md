# Mobazha Tray

System tray application for Mobazha standalone store. Provides a native desktop experience for non-technical users on macOS and Windows.

## Architecture

`mobazha-tray` is a thin GUI wrapper around the `mobazha` CLI binary:

```
mobazha-tray (CGO_ENABLED=1, native compile)
  └── exec → mobazha start (CGO_ENABLED=0, cross-compiled)
```

The tray binary must be built natively (not cross-compiled) because `fyne-io/systray` requires CGO for platform-specific system tray APIs (Cocoa on macOS, Win32 on Windows).

## Features

- System tray icon with color-coded status (green=running, orange=starting, gray=stopped)
- Auto-starts the Mobazha node on launch
- Auto-opens the Web UI in the default browser once the node is healthy
- Menu: Open Store, Status, Start/Stop Node, Quit
- Graceful shutdown on quit (sends SIGINT, waits 10s, then SIGKILL)

## Building

```bash
# macOS (on a Mac)
CGO_ENABLED=1 go build -o mobazha-tray ./cmd/mobazha-tray

# Windows (on a Windows machine or with proper cross-compile toolchain)
CGO_ENABLED=1 GOOS=windows go build -o mobazha-tray.exe ./cmd/mobazha-tray
```

## Binary Discovery

The tray program looks for the `mobazha` binary in this order:
1. Same directory as the tray executable
2. System PATH

## Icon Assets

| File | Size | Usage |
|---|---|---|
| `assets/icon.svg` | Vector source | Editable source (Mobazha "M" metaball logo) |
| `assets/icon-1024.png` | 1024x1024 | Master raster, used to derive all sizes |
| `assets/icon.png` | 128x128 | Tray icon — Running state (green) |
| `assets/icon-starting.png` | 128x128 | Tray icon — Starting state (orange) |
| `assets/icon-stopped.png` | 128x128 | Tray icon — Stopped state (gray) |
| `assets/Mobazha.icns` | Multi-res | macOS `.app` bundle icon (16–1024px + @2x) |

To regenerate from the SVG source, use Chrome headless + `sips` + `iconutil` (see `scripts/build-macos-app.sh`).

## Distribution

### macOS (curl install)

macOS tray is distributed via `install.sh` (bypasses Gatekeeper since `curl` doesn't add quarantine attribute):

```bash
curl -sSL https://get.mobazha.org/install | bash
# Installs both mobazha (CLI) and mobazha-tray to ~/.local/bin/
```

### Windows

Download `.zip` from GitHub Release, extract, double-click `mobazha-tray.exe`.

## Release Process

### Automated (CI)

- CLI binaries (all platforms) and Windows tray are built by `release-native.yml`
- macOS tray is **not** built in CI (no macOS runners to minimize costs)

### Manual (macOS tray)

After CI creates a release, build and upload macOS tray locally:

```bash
cd ~/go/src/github.com/mobazha/mobazha3.0

# Build both architectures
GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o /tmp/mobazha-tray-darwin-arm64 ./cmd/mobazha-tray
GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o /tmp/mobazha-tray-darwin-amd64 ./cmd/mobazha-tray

# Upload to release
gh release upload native-<VERSION> \
  /tmp/mobazha-tray-darwin-arm64 \
  /tmp/mobazha-tray-darwin-amd64 \
  --repo mobazha/mobazha.org

# Update checksums
gh release download native-<VERSION> --repo mobazha/mobazha.org \
  --pattern "checksums-sha256.txt" --dir /tmp --clobber
cd /tmp && shasum -a 256 mobazha-tray-darwin-arm64 mobazha-tray-darwin-amd64 >> checksums-sha256.txt
gh release upload native-<VERSION> /tmp/checksums-sha256.txt \
  --repo mobazha/mobazha.org --clobber
```

## Packaging (legacy, requires Apple Developer)

- **macOS**: Bundle into `Mobazha.app` (see `scripts/build-macos-app.sh`) — requires code signing + notarization
- **Windows**: Bundle into `.zip` (see `scripts/build-windows-zip.sh`)
