package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"awesomeProject/packaging/metadata"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if len(arguments) == 0 || arguments[0] == "help" || arguments[0] == "--help" || arguments[0] == "-h" {
		printUsage()
		return nil
	}

	command := arguments[0]
	switch command {
	case "version":
		fmt.Println(metadata.Current.Version)
		return nil
	case "metadata":
		return generateMetadata(root, false)
	case "check":
		return generateMetadata(root, true)
	case "auto":
		command, err = nativeTarget()
		if err != nil {
			return err
		}
	case "macos", "windows", "flatpak":
	default:
		return fmt.Errorf("unknown command %q (run with --help)", command)
	}

	if err := requireNativeHost(command); err != nil {
		return err
	}
	if err := generateMetadata(root, false); err != nil {
		return err
	}
	return runPlatformBuild(root, command, arguments[1:])
}

func printUsage() {
	fmt.Printf(`FilaTrack cross-platform build frontend

Usage:
  go run ./cmd/build auto [platform options]
  go run ./cmd/build macos [--dmg] [--install-deps] [--sign IDENTITY]
  go run ./cmd/build windows [-InstallDependencies] [-Archive] [-Installer]
  go run ./cmd/build flatpak
  go run ./cmd/build version
  go run ./cmd/build metadata
  go run ./cmd/build check

Version and application metadata: packaging/metadata/app.json
`)
}

func projectRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("could not find project root containing go.mod")
		}
		directory = parent
	}
}

func nativeTarget() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "macos", nil
	case "windows":
		return "windows", nil
	case "linux":
		return "flatpak", nil
	default:
		return "", fmt.Errorf("unsupported build host %s", runtime.GOOS)
	}
}

func requireNativeHost(target string) error {
	want := map[string]string{"macos": "darwin", "windows": "windows", "flatpak": "linux"}[target]
	if runtime.GOOS != want {
		return fmt.Errorf("%s packages must be built on %s (current host: %s)", target, want, runtime.GOOS)
	}
	return nil
}

func runPlatformBuild(root, target string, arguments []string) error {
	var command *exec.Cmd
	switch target {
	case "macos":
		command = exec.Command("bash", append([]string{filepath.Join(root, "packaging", "scripts", "build-macos.sh")}, arguments...)...)
	case "flatpak":
		command = exec.Command("bash", append([]string{filepath.Join(root, "packaging", "scripts", "build-flatpak.sh")}, arguments...)...)
	case "windows":
		command = exec.Command("powershell.exe", append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(root, "packaging", "scripts", "build-windows.ps1")}, arguments...)...)
	default:
		return fmt.Errorf("unsupported build target %s", target)
	}
	app := metadata.Current
	command.Dir = root
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = append(os.Environ(),
		"FILATRACK_BUILD_MANAGED=1",
		"FILATRACK_APP_ID="+app.ID,
		"FILATRACK_APP_NAME="+app.Name,
		"FILATRACK_EXECUTABLE="+app.Executable,
		"FILATRACK_VERSION="+app.Version,
		"FILATRACK_WINDOWS_VERSION="+app.WindowsVersion(),
		"FILATRACK_MACOS_MIN_VERSION="+app.MacOS.MinimumVersion,
		"FILATRACK_FLATPAK_RUNTIME_VERSION="+app.Flatpak.RuntimeVersion,
		"FILATRACK_GOLANG_SDK_VERSION="+app.Flatpak.GolangSDKVersion,
	)
	return command.Run()
}

func generateMetadata(root string, check bool) error {
	files, err := renderedMetadata()
	if err != nil {
		return err
	}
	stale := make([]string, 0)
	for relativePath, contents := range files {
		path := filepath.Join(root, relativePath)
		current, readErr := os.ReadFile(path)
		if readErr == nil && bytes.Equal(current, contents) {
			continue
		}
		if check {
			stale = append(stale, relativePath)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			return err
		}
		fmt.Println("generated", relativePath)
	}
	if len(stale) > 0 {
		return fmt.Errorf("generated metadata is stale: %s (run: go run ./cmd/build metadata)", strings.Join(stale, ", "))
	}
	return nil
}

func renderedMetadata() (map[string][]byte, error) {
	app := metadata.Current
	metainfo, err := renderMetainfo(app)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		filepath.Join("packaging", app.ID+".desktop"):      []byte(renderDesktop(app)),
		filepath.Join("packaging", app.ID+".metainfo.xml"): metainfo,
		"filatrack.flatpak.yml":                            []byte(renderFlatpak(app)),
	}, nil
}

func renderMetainfo(app metadata.App) ([]byte, error) {
	escape := func(value string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(value)
	}
	contents := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!-- Generated from packaging/metadata/app.json; do not edit. -->
<component type="desktop-application">
  <id>%s</id>
  <name>%s</name>
  <summary>%s</summary>
  <summary xml:lang="ru">%s</summary>
  <metadata_license>%s</metadata_license>
  <project_license>%s</project_license>
  <developer id="%s">
    <name>%s</name>
  </developer>
  <url type="homepage">%s</url>
  <description>
    <p>%s</p>
    <p xml:lang="ru">%s</p>
  </description>
  <launchable type="desktop-id">%s.desktop</launchable>
  <provides>
    <binary>%s</binary>
  </provides>
  <content_rating type="oars-1.1"/>
  <releases>
    <release version="%s" date="%s"/>
  </releases>
</component>
`, escape(app.ID), escape(app.Name), escape(app.Summary.English), escape(app.Summary.Russian),
		escape(app.MetadataLicense), escape(app.ProjectLicense), escape(app.Developer.ID), escape(app.Developer.Name), escape(app.Homepage), escape(app.Description.English),
		escape(app.Description.Russian), escape(app.ID), escape(app.Executable), escape(app.Version), escape(app.ReleaseDate))
	return []byte(contents), nil
}

func renderDesktop(app metadata.App) string {
	return fmt.Sprintf(`# Generated from packaging/metadata/app.json; do not edit.
[Desktop Entry]
Type=Application
Name=%s
Comment=%s
Comment[ru]=%s
Exec=%s
Icon=%s
Terminal=false
Categories=%s;
Keywords=%s;
StartupNotify=true
StartupWMClass=%s
`, app.Name, app.Summary.English, app.Summary.Russian, app.Executable, app.ID,
		strings.Join(app.Categories, ";"), strings.Join(app.Keywords, ";"), app.ID)
}

func renderFlatpak(app metadata.App) string {
	return fmt.Sprintf(`# Generated from packaging/metadata/app.json; do not edit.
app-id: %s
runtime: org.gnome.Platform
runtime-version: '%s'
sdk: org.gnome.Sdk
command: %s

sdk-extensions:
  - org.freedesktop.Sdk.Extension.golang

finish-args:
  - --share=ipc
  - --share=network
  - --socket=wayland
  - --socket=fallback-x11
  - --device=dri
  - --env=FILATRACK_DATA_DIR=/var/data

modules:
  - name: %s
    buildsystem: simple
    build-options:
      append-path: /usr/lib/sdk/golang/bin
      env:
        CGO_ENABLED: '1'
        GOFLAGS: -mod=vendor
    build-commands:
      - go build -trimpath -ldflags=-s -o %s ./cmd/filatrack
      - install -Dm755 %s /app/bin/%s
      - install -Dm644 packaging/%s.desktop /app/share/applications/%s.desktop
      - install -Dm644 packaging/%s.metainfo.xml /app/share/metainfo/%s.metainfo.xml
      - install -Dm644 packaging/%s.svg /app/share/icons/hicolor/scalable/apps/%s.svg
    sources:
      - type: dir
        path: .
        skip:
          - .flatpak-builder
          - build-dir
          - repo
          - dist
          - '*.db'
          - '*.db-shm'
          - '*.db-wal'
          - cache
`, app.ID, app.Flatpak.RuntimeVersion, app.Executable, app.Executable,
		app.Executable, app.Executable, app.Executable,
		app.ID, app.ID, app.ID, app.ID, app.ID, app.ID)
}
