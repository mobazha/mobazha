# Mobazha Launcher

Supervisor + auto-update manager for the Mobazha standalone store. Manages the `mobazha` node process lifecycle, monitors health, handles crash recovery with exponential backoff, and performs automatic binary updates from GitHub Releases.

## Architecture

```
mobazha-launcher (Supervisor)
  ├── Process Manager    — start/stop/restart mobazha node
  ├── Health Monitor     — poll /healthz every 5s
  ├── Crash Recovery     — exponential backoff (5s → 5min max)
  ├── Update Checker     — query GitHub Releases API
  ├── Downloader         — fetch binary + SHA256 verify
  ├── Atomic Replacer    — rename swap + rollback on failure
  ├── Config Manager     — watch launcher-config.json (30s poll)
  ├── Status Writer      — write update-status.json for Node
  ├── Trigger Watcher    — poll update-trigger.json from Node
  └── [desktop] Systray  — macOS/Windows system tray UI
```

Two build modes controlled by Go build tags:

| Mode | Build Tag | CGO | Platforms | UI |
|---|---|---|---|---|
| Desktop | `desktop` | 1 | macOS, Windows | System tray (fyne-io/systray) |
| Headless | `!desktop` (default) | 0 | Linux | No UI, signal-based shutdown |

## File-based IPC

Launcher and Node communicate through JSON files in `~/.mobazha/`:

| File | Writer | Reader | Purpose |
|---|---|---|---|
| `update-status.json` | Launcher | Node | Update status, versions, progress |
| `update-trigger.json` | Node | Launcher | User-initiated check/apply actions |
| `launcher-config.json` | Node (via API) | Launcher | Auto-update settings |

## Building

```bash
# Headless (Linux servers, CGO=0)
CGO_ENABLED=0 go build \
  -ldflags "-X github.com/mobazha/mobazha3.0/internal/supervisor.Version=v0.1.0" \
  -o mobazha-launcher ./cmd/mobazha-launcher

# Desktop (macOS, must build natively)
CGO_ENABLED=1 go build -tags desktop \
  -ldflags "-s -w -X github.com/mobazha/mobazha3.0/internal/supervisor.Version=v0.1.0" \
  -o mobazha-launcher ./cmd/mobazha-launcher
```

## Binary Discovery

The launcher looks for the `mobazha` binary in this order:
1. Same directory as the launcher executable
2. System PATH

## Icon Assets (Desktop only)

| File | Size | Usage |
|---|---|---|
| `assets/icon.svg` | Vector source | Editable source (Mobazha "M" metaball logo) |
| `assets/icon-1024.png` | 1024x1024 | Master raster |
| `assets/icon.png` | 128x128 | Tray icon — Running (green) |
| `assets/icon-starting.png` | 128x128 | Tray icon — Starting (orange) |
| `assets/icon-stopped.png` | 128x128 | Tray icon — Stopped (gray) |
| `assets/Mobazha.icns` | Multi-res | macOS .app bundle icon |

## Distribution

### macOS (curl install)

```bash
curl -sSL https://get.mobazha.org/install | bash
# Installs both mobazha (CLI) and mobazha-launcher to ~/.local/bin/
```

### Windows

Download `.zip` from GitHub Release, extract, double-click `mobazha-launcher.exe`.

### Linux

```bash
curl -sSL https://get.mobazha.org/install | bash
# Installs mobazha (CLI) + mobazha-launcher (headless) to ~/.local/bin/
# Optionally register as systemd service: mobazha service install
```

## Testing

### Environment variable override

Set `MOBAZHA_UPDATE_URL` to point the update checker at a local mock server:

```bash
MOBAZHA_UPDATE_URL=http://127.0.0.1:9999/ mobazha-launcher
```

### Mock release server

```bash
go run scripts/test/fake-release-server.go -binary /path/to/new-binary -version 99.0.0
```

## Release Process

### Automated (CI)

- CLI binaries (all platforms) built by `release-native.yml`
- Linux + Windows launcher binaries built by CI
- macOS launcher requires local build (no CI macOS runners)

### Manual (macOS launcher)

```bash
cd ~/go/src/github.com/mobazha/mobazha3.0

GOARCH=arm64 CGO_ENABLED=1 go build -tags desktop -ldflags="-s -w" \
  -o /tmp/mobazha-launcher-darwin-arm64 ./cmd/mobazha-launcher
GOARCH=amd64 CGO_ENABLED=1 go build -tags desktop -ldflags="-s -w" \
  -o /tmp/mobazha-launcher-darwin-amd64 ./cmd/mobazha-launcher

gh release upload native-<VERSION> \
  /tmp/mobazha-launcher-darwin-arm64 \
  /tmp/mobazha-launcher-darwin-amd64 \
  --repo mobazha/mobazha.org

gh release download native-<VERSION> --repo mobazha/mobazha.org \
  --pattern "checksums-sha256.txt" --dir /tmp --clobber
cd /tmp && shasum -a 256 mobazha-launcher-darwin-arm64 mobazha-launcher-darwin-amd64 \
  >> checksums-sha256.txt
gh release upload native-<VERSION> /tmp/checksums-sha256.txt \
  --repo mobazha/mobazha.org --clobber
```

## Design Documents

- [MOBAZHA_LAUNCHER.md](../../docs/deploy/MOBAZHA_LAUNCHER.md) — Launcher architecture
- [NATIVE_AUTO_UPDATE_DESIGN.md](../../docs/deploy/NATIVE_AUTO_UPDATE_DESIGN.md) — Auto-update protocol and UI
