package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

type DatabaseSelection struct {
	Driver      string `json:"driver"`
	SQLitePath  string `json:"sqlite_path,omitempty"`
	PostgresURL string `json:"postgres_url,omitempty"`
}

func (s DatabaseSelection) Validate() error {
	switch s.Driver {
	case DriverSQLite:
		return nil
	case DriverPostgres:
		if s.PostgresURL != "" {
			return nil
		}
		return fmt.Errorf("PostgreSQL connection URL is required")
	default:
		return fmt.Errorf("unsupported database driver %q", s.Driver)
	}
}

func LoadDatabaseSelection(dataDir string) (DatabaseSelection, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, "database.json"))
	if os.IsNotExist(err) {
		return DatabaseSelection{Driver: DriverSQLite}, nil
	}
	if err != nil {
		return DatabaseSelection{}, err
	}
	var selection DatabaseSelection
	if err := json.Unmarshal(data, &selection); err != nil {
		return selection, err
	}
	return selection, selection.Validate()
}

func SaveDatabaseSelection(dataDir string, selection DatabaseSelection) error {
	if err := selection.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		return err
	}
	name := filepath.Join(dataDir, "database.json")
	tmp, err := os.CreateTemp(dataDir, ".database-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}

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
