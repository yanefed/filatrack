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
	TotalPrints          string
	SuccessRate          string
	TotalUsed            string
	AvgPerPrint          string
	NoDryingWarnings     string
	SpoolNeedsDrying     string
	DryingWarnings       string
	StatsUnavailable     string
	ActivePrints         string
	NoActivePrints       string
	RecentPrints         string
	RemainingFilamentFmt string
	MoreItemsFmt         string

	// Spools
	Save               string
	Cancel             string
	Archive            string
	AddSpoolBtn        string
	ManufacturerPH     string
	MaterialLabel      string
	ColorPH            string
	ColorHexLabel      string
	NetWeightPH        string
	PricePH            string
	AdditionalSettings string
	FilamentDiameter   string
	FilamentDensity    string
	SpoolInfoFmt       string
	SpoolWeightFmt     string

	// Prints
	PrintCostFmt   string
	CostNotSet     string
	CurrencyLabel  string
	CurrencyHelp   string
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
	NoDryingHistory       string
	AddDrying             string
	TemperatureC          string
	DurationHours         string
	MaxDaysNoDry          string
	LastDryingFmt         string
	NeverDried            string
	NextDryingFmt         string
	DryingDueToday        string
	DryingOverdueFmt      string
	DryingRequired        string
	NoDryingProfile       string
	DrySpool              string

	// Settings
	LanguageLabel    string
	Print            string
	MaterialSettings string
	AddMaterial      string
	MaterialName     string
	Days             string
	Hours            string
	Edit             string
	Delete           string
	Close            string

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
	PrinterIdleInfoFmt string
	CurrentFileFmt     string
	AddPrinterError    string
	AssignPrinterError string
	CheckConnection    string
	ConnectionOK       string
	ConnectionFailed   string
	RefreshHistory     string
	AssignHistoryJob   string
	UploadModel        string
	NoFileSelected     string
	StartAfterUpload   string
	UploadAndPrint     string
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
	LowFilamentLevel   string

	// About
	AboutProgram string
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
