# FilaTrack

GTK4/libadwaita desktop app for filament inventory, drying history, print history, and Moonraker printer tracking.

What it does:

- keeps a catalog of spools with remaining weight, price, material, and color
- stores print history and basic usage stats
- tracks drying sessions and warns when a spool should be dried again
- connects to Moonraker printers, shows status, progress, current file, and syncs print history

## Stack

- Go
- GTK4 + libadwaita
- SQLite
- Moonraker API / WebSocket

## Run

Linux desktop with GTK4/libadwaita development packages installed:

```bash
go run ./cmd/filatrack
```

By default the app stores data in the current directory. To move the database and cache elsewhere:

```bash
FILATRACK_DATA_DIR=/path/to/data go run ./cmd/filatrack
```

## Build

macOS application bundle (requires Homebrew):

```bash
./build-macos.sh --install-deps --dmg
```

The script creates `dist/FilaTrack.app`, bundles the GTK runtime, applies ad-hoc
signing by default, and optionally creates a DMG. For distribution, pass an Apple
Developer ID with `--sign`.

Windows bundle:

```powershell
.\build-windows.ps1 -InstallDependencies -Archive
```

Flatpak:

```bash
./build-flatpak.sh
```
