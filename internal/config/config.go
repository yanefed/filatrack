package config

import "path/filepath"

type Config struct {
	DBPath  string
	DataDir string
}

func Default() *Config {
	dataDir := "."
	return &Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "filatrack.db"),
	}
}
