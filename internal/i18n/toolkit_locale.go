package i18n

/*
#include <locale.h>
#include <stdlib.h>

static char *filatrack_setlocale_messages(const char *locale) {
#ifdef LC_MESSAGES
	return setlocale(LC_MESSAGES, locale);
#else
	// The Windows UCRT has no LC_MESSAGES category. LC_ALL is the only
	// available category that also lets gettext initialize its locale.
	return setlocale(LC_ALL, locale);
#endif
}
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

// ConfigureToolkitLanguage synchronizes gettext-based GTK/libadwaita strings
// with the language selected inside FilaTrack.
func ConfigureToolkitLanguage(lang Language) error {
	candidates, ok := toolkitLocaleCandidates[lang]
	if !ok {
		return fmt.Errorf("unsupported toolkit language: %s", lang)
	}
	if err := os.Setenv("LANGUAGE", string(lang)); err != nil {
		return fmt.Errorf("setting LANGUAGE: %w", err)
	}

	for _, candidate := range candidates {
		localeName := C.CString(candidate)
		configured := C.filatrack_setlocale_messages(localeName) != nil
		C.free(unsafe.Pointer(localeName))
		if configured {
			return nil
		}
	}
	return fmt.Errorf("no installed locale found for language %s", lang)
}

var toolkitLocaleCandidates = map[Language][]string{
	LangEN: {"en_US.UTF-8", "en_GB.UTF-8", "English_United States.65001", "C.UTF-8", "C.utf8", "C"},
	LangBG: {"bg_BG.UTF-8", "bg_BG.utf8", "Bulgarian_Bulgaria.65001"},
	LangRU: {"ru_RU.UTF-8", "ru_RU.utf8", "Russian_Russia.65001"},
}
