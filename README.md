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
- SQLite or PostgreSQL
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

Application identity, version, localized descriptions, licenses, and platform
versions are configured in `packaging/metadata/app.json`. Generate/check derived
Flatpak and AppStream files with:

```bash
go run ./cmd/build metadata
go run ./cmd/build check
go run ./cmd/build version
```

Build the package native to the current host with `auto`, or select it explicitly:

```bash
go run ./cmd/build auto
go run ./cmd/build macos --install-deps --dmg
go run ./cmd/build windows -InstallDependencies -Archive -Installer
go run ./cmd/build flatpak
```
