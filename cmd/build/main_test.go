package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yanefed/filatrack/packaging/metadata"
)

func TestRenderedMetadataUsesCanonicalValues(t *testing.T) {
	files, err := renderedMetadata()
	if err != nil {
		t.Fatal(err)
	}
	app := metadata.Current
	for path, contents := range files {
		text := string(contents)
		if !strings.Contains(text, app.ID) {
			t.Errorf("%s does not contain app ID %q", path, app.ID)
		}
	}
	metainfo := string(files[filepath.Join("packaging", app.ID+".metainfo.xml")])
	if !strings.Contains(metainfo, `xml:lang="ru"`) || !strings.Contains(metainfo, `version="`+app.Version+`"`) {
		t.Fatalf("generated metainfo is missing localization or version:\n%s", metainfo)
	}
}

func TestGenerateMetadataAndCheck(t *testing.T) {
	root := t.TempDir()
	if err := generateMetadata(root, false); err != nil {
		t.Fatal(err)
	}
	if err := generateMetadata(root, true); err != nil {
		t.Fatalf("fresh metadata failed check: %v", err)
	}
	path := filepath.Join(root, "filatrack.flatpak.yml")
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generateMetadata(root, true); err == nil {
		t.Fatal("stale metadata passed check")
	}
}
