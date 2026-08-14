//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"awesomeProject/packaging/metadata"
)

// configureBundleEnvironment makes the native binary self-sufficient when it
// is launched from Contents/MacOS inside a macOS application bundle.
func configureBundleEnvironment() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}

	macOSDir := filepath.Dir(executable)
	contentsDir := filepath.Dir(macOSDir)
	if filepath.Base(macOSDir) != "MacOS" || filepath.Base(contentsDir) != "Contents" {
		return
	}

	resourcesDir := filepath.Join(contentsDir, "Resources")
	dataDir := os.Getenv("FILATRACK_DATA_DIR")
	if dataDir == "" {
		if userConfigDir, configErr := os.UserConfigDir(); configErr == nil {
			dataDir = filepath.Join(userConfigDir, metadata.Current.Name)
			_ = os.Setenv("FILATRACK_DATA_DIR", dataDir)
		}
	}

	setDefaultEnv("GSETTINGS_SCHEMA_DIR", filepath.Join(resourcesDir, "share", "glib-2.0", "schemas"))
	setDefaultEnv("GIO_EXTRA_MODULES", filepath.Join(resourcesDir, "lib", "gio", "modules"))
	setDefaultEnv("FONTCONFIG_PATH", filepath.Join(resourcesDir, "etc", "fonts"))
	setDefaultEnv("XDG_DATA_DIRS", filepath.Join(resourcesDir, "share"))

	loaderDir := filepath.Join(resourcesDir, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders")
	setDefaultEnv("GDK_PIXBUF_MODULEDIR", loaderDir)
	if dataDir == "" {
		return
	}
	loaderCache := filepath.Join(dataDir, "gdk-pixbuf-loaders.cache")
	setDefaultEnv("GDK_PIXBUF_MODULE_FILE", loaderCache)
	if err := writePixbufLoaderCache(filepath.Join(macOSDir, "gdk-pixbuf-query-loaders"), loaderDir, loaderCache); err != nil {
		fmt.Fprintf(os.Stderr, "configuring bundled image loaders: %v\n", err)
	}
}

func setDefaultEnv(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}

func writePixbufLoaderCache(queryTool, loaderDir, cachePath string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("creating application data directory: %w", err)
	}
	command := exec.Command(queryTool)
	command.Env = append(os.Environ(), "GDK_PIXBUF_MODULEDIR="+loaderDir)
	contents, err := command.Output()
	if err != nil {
		return fmt.Errorf("querying image loaders: %w", err)
	}
	temporaryPath := cachePath + ".tmp"
	if err := os.WriteFile(temporaryPath, contents, 0o644); err != nil {
		return fmt.Errorf("writing image loader cache: %w", err)
	}
	if err := os.Rename(temporaryPath, cachePath); err != nil {
		return fmt.Errorf("installing image loader cache: %w", err)
	}
	return nil
}
