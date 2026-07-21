package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DBPath  string
	DataDir string
}

func Default() *Config {
	dataDir := "."
	if configured := os.Getenv("FILATRACK_DATA_DIR"); configured != "" {
		dataDir = configured
	}
	return &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "filatrack.db"),
	}
}
