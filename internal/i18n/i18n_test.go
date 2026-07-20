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

func TestSupportedLanguages(t *testing.T) {
	languages := SupportedLanguages()
	if len(languages) != 2 {
		t.Fatalf("supported language count = %d, want 2", len(languages))
	}
	if languages[0].Code != LangEN || languages[0].Name != "English" {
		t.Fatalf("unexpected English language entry: %+v", languages[0])
	}
	if languages[1].Code != LangBG || languages[1].Name != "Български" {
		t.Fatalf("unexpected Bulgarian language entry: %+v", languages[1])
	}
}
