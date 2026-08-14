package metadata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type LocalizedText struct {
	English string `json:"en"`
	Russian string `json:"ru"`
}

type MacOS struct {
	MinimumVersion string `json:"minimum_version"`
}

type Flatpak struct {
	RuntimeVersion   string `json:"runtime_version"`
	GolangSDKVersion string `json:"golang_sdk_version"`
}

type Developer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type App struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Executable      string        `json:"executable"`
	Version         string        `json:"version"`
	ReleaseDate     string        `json:"release_date"`
	Summary         LocalizedText `json:"summary"`
	Description     LocalizedText `json:"description"`
	ProjectLicense  string        `json:"project_license"`
	MetadataLicense string        `json:"metadata_license"`
	Homepage        string        `json:"homepage"`
	Developer       Developer     `json:"developer"`
	Categories      []string      `json:"categories"`
	Keywords        []string      `json:"keywords"`
	MacOS           MacOS         `json:"macos"`
	Flatpak         Flatpak       `json:"flatpak"`
}

//go:embed app.json
var raw []byte

var Current = mustLoad()

func mustLoad() App {
	var app App
	if err := json.Unmarshal(raw, &app); err != nil {
		panic(fmt.Sprintf("invalid embedded application metadata: %v", err))
	}
	if err := app.Validate(); err != nil {
		panic(fmt.Sprintf("invalid embedded application metadata: %v", err))
	}
	return app
}

func (a App) Validate() error {
	required := map[string]string{
		"id": a.ID, "name": a.Name, "executable": a.Executable,
		"version": a.Version, "release_date": a.ReleaseDate,
		"summary.en": a.Summary.English, "description.en": a.Description.English,
		"project_license": a.ProjectLicense, "metadata_license": a.MetadataLicense,
		"homepage": a.Homepage, "developer.id": a.Developer.ID, "developer.name": a.Developer.Name,
		"macos.minimum_version":      a.MacOS.MinimumVersion,
		"flatpak.runtime_version":    a.Flatpak.RuntimeVersion,
		"flatpak.golang_sdk_version": a.Flatpak.GolangSDKVersion,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must fit on one line", field)
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+([.-][A-Za-z0-9_-]+)+$`).MatchString(a.ID) {
		return fmt.Errorf("id %q is not a valid application ID", a.ID)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(a.Executable) {
		return fmt.Errorf("executable %q is not a safe filename", a.Executable)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(a.Version) {
		return fmt.Errorf("version %q must use MAJOR.MINOR.PATCH", a.Version)
	}
	if _, err := time.Parse("2006-01-02", a.ReleaseDate); err != nil {
		return fmt.Errorf("release_date %q must use YYYY-MM-DD", a.ReleaseDate)
	}
	return nil
}

func (a App) WindowsVersion() string {
	return a.Version + ".0"
}
