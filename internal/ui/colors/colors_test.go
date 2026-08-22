package colors

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

func TestToHex(t *testing.T) {
	color := gdk.NewRGBA(1, 0.5, 0, 1)
	if got := ToHex(&color); got != "#ff8000" {
		t.Fatalf("ToHex() = %q, want #ff8000", got)
	}
	if got := ToHex(nil); got != DefaultHex {
		t.Fatalf("ToHex(nil) = %q, want %s", got, DefaultHex)
	}
}

func TestHexRoundTrip(t *testing.T) {
	if got := ToHex(FromHex("#1a2b3c")); got != "#1a2b3c" {
		t.Fatalf("hex round trip = %q, want #1a2b3c", got)
	}
}

func TestFromHexUsesDefaultForInvalidValue(t *testing.T) {
	if got := ToHex(FromHex("not-a-color")); got != DefaultHex {
		t.Fatalf("invalid color = %q, want %s", got, DefaultHex)
	}
}
