package i18n

import "testing"

func TestLanguageSelectionAndFallback(t *testing.T) {
	original := GetLanguage()
	t.Cleanup(func() { SetLanguage(original) })

	SetLanguage(LangBG)
	if got := GetLanguage(); got != LangBG {
		t.Fatalf("GetLanguage() = %q, want %q", got, LangBG)
	}
	if got := L().Dashboard; got != "Табло" {
		t.Fatalf("Bulgarian dashboard translation = %q", got)
	}

	SetLanguage(Language("unsupported"))
	if got := L().Dashboard; got != translations[LangEN].Dashboard {
		t.Fatalf("fallback dashboard translation = %q, want English", got)
	}
}
