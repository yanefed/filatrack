package metadata

import "testing"

func TestEmbeddedMetadataIsValid(t *testing.T) {
	if err := Current.Validate(); err != nil {
		t.Fatal(err)
	}
	if Current.WindowsVersion() != Current.Version+".0" {
		t.Fatalf("WindowsVersion() = %q", Current.WindowsVersion())
	}
}

func TestValidateRejectsNonSemanticVersion(t *testing.T) {
	app := Current
	app.Version = "next"
	if err := app.Validate(); err == nil {
		t.Fatal("Validate() accepted a non-semantic version")
	}
}
