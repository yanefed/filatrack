package i18n

import "sync"

// Language represents a supported language.
type Language string

const (
	LangEN Language = "en"
	LangBG Language = "bg"
	LangRU Language = "ru"
)

// T holds all translatable strings.
type T struct {
	// Navigation
	Dashboard string
	Spools    string
	Prints    string
	Drying    string
	Settings  string

	// Dashboard
	TotalPrints      string
	SuccessRate      string
	TotalUsed        string
	AvgPerPrint      string
	NoDryingWarnings string
	SpoolNeedsDrying string
	DryingWarnings   string
	StatsUnavailable string

	// Spools
	SpoolsTitle    string
	Save           string
	Cancel         string
	Archive        string
	AddSpoolBtn    string
	ManufacturerPH string
	ColorPH        string
	NetWeightPH    string
	PricePH        string
	SpoolInfoFmt   string

	// Prints
	SelectGCode    string
	ErrorPrefix    string
	UsageFormat    string
	NoPrintRecords string
	SpoolColon     string
	NoSpoolsAvail  string
	SaveError      string

	// Drying
	DryingRecommendations string
	DryingLog             string
	AddDrying             string
	TemperatureC          string
	DurationHours         string
	MaxDaysNoDry          string

	// Settings
	LanguageLabel    string
	DryingProfiles   string
	CustomMaterials  string
	AddMaterial      string
	MaterialName     string
	BuiltInMaterial  string
	Days             string
	Hours            string
	Edit             string
	Delete           string
	Close            string
	DryingRule       string
	UseDryingRule    string
	CreateDryingRule string
	NoDryingRule     string

	// Printers
	Printers           string
	AddPrinter         string
	PrinterName        string
	MoonrakerURL       string
	APIKey             string
	ActiveSpool        string
	NoPrinters         string
	UnassignedSpool    string
	SaveAssignment     string
	PrinterInfoFmt     string
	CurrentFileFmt     string
	AddPrinterError    string
	AssignPrinterError string
	CheckConnection    string
	ConnectionOK       string
	ConnectionFailed   string
	RefreshHistory     string
	AssignHistoryJob   string
	UploadAndPrint     string
	LocalGCodePath     string
	LaunchBlocked      string
	LaunchWarning      string
	LaunchStarted      string
	LaunchUploaded     string
	Models             string
	RefreshModels      string
	NoPrinterModels    string
	PrintModel         string
	PrintModelDialog   string
	LowFilamentSetting string
}

var translations = map[Language]T{
	LangEN: langEn,
	LangBG: langBg,
	LangRU: langRu,
}

var (
	currentLang = LangEN
	mu          sync.RWMutex
)

// SetLanguage sets the current language.
func SetLanguage(lang Language) {
	mu.Lock()
	defer mu.Unlock()
	currentLang = lang
}

// GetLanguage returns the current language.
func GetLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// L returns the translations for the current language.
func L() T {
	mu.RLock()
	defer mu.RUnlock()
	if t, ok := translations[currentLang]; ok {
		return t
	}
	return translations[LangEN]
}

// SupportedLanguages returns available languages with display names.
func SupportedLanguages() []struct {
	Code Language
	Name string
} {
	return []struct {
		Code Language
		Name string
	}{
		{LangEN, "English"},
		{LangBG, "Български"},
		{LangRU, "Русский"},
	}
}
func IsSupportedLanguage(lang Language) bool {
	_, ok := translations[lang]
	return ok
}
