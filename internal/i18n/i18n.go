package i18n

import "sync"

// Language represents a supported language.
type Language string

const (
	LangEN Language = "en"
	LangBG Language = "bg"
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
	DashboardTitle   string
	TotalSpools      string
	TotalPrints      string
	SuccessRate      string
	TotalUsed        string
	AvgPerPrint      string
	NoDryingWarnings string
	SpoolNeedsDrying string
	DaysAgo          string
	NeverDried       string
	DryingWarnings   string
	NoSpools         string
	StatsUnavailable string

	// Spools
	SpoolsTitle  string
	AddSpool     string
	Manufacturer string
	Material     string
	Color        string
	ColorHex     string
	DiameterMM   string
	WeightG      string
	Price        string
	PurchaseDate string
	Save         string
	Cancel       string
	Remaining    string
	Archived     string
	Archive      string

	// Prints
	PrintsTitle       string
	AddPrint          string
	SelectGCode       string
	SelectSpool       string
	Status            string
	StatusSuccess     string
	StatusFailed      string
	StatusCancelled   string
	CompletionPercent string
	Notes             string
	PrintDate         string
	EstimatedWeight   string
	ActualWeight      string
	Duration          string
	NozzleTemp        string
	BedTemp           string
	Slicer            string

	// Drying
	DryingTitle           string
	DryingRecommendations string
	DryingLog             string
	AddDrying             string
	TemperatureC          string
	DurationHours         string
	MaxDaysNoDry          string

	// Prints (extra)
	ErrorPrefix    string
	UsageFormat    string
	CompletionFmt  string
	NoPrintRecords string
	GCodeFilePath  string
	LoadGCode      string
	SpoolColon     string
	NoSpoolsAvail  string
	StatusColon    string
	CompletionPct  string
	EnterFilePath  string
	ParseError     string
	SaveError      string
	ModelInfo      string

	// Spools (extra)
	SpoolsScreenTitle string
	AddSpoolBtn       string
	ManufacturerPH    string
	ColorPH           string
	NetWeightPH       string
	PricePH           string
	SpoolInfoFmt      string

	// Settings
	SettingsTitle   string
	LanguageLabel   string
	DryingProfiles  string
	CustomMaterials string
	AddMaterial     string
	DeleteMaterial  string
	MaterialName    string
	SaveSettings    string
	BuiltInMaterial string
	DryingFrequency string
	Days            string
	Hours           string
	Edit            string
	Delete          string
	Add             string
	Close           string
	RestartNote     string
}

var translations = map[Language]T{
	LangEN: {
		Dashboard:             "Dashboard",
		Spools:                "Spools",
		Prints:                "Prints",
		Drying:                "Drying",
		Settings:              "Settings",
		DashboardTitle:        "Dashboard",
		TotalSpools:           "Total spools",
		TotalPrints:           "Total prints",
		SuccessRate:           "Success rate",
		TotalUsed:             "Total used",
		AvgPerPrint:           "Avg per print",
		NoDryingWarnings:      "No drying warnings",
		SpoolNeedsDrying:      "needs drying",
		DaysAgo:               "days ago",
		NeverDried:            "never dried",
		DryingWarnings:        "Drying warnings",
		NoSpools:              "No spools",
		StatsUnavailable:      "Statistics unavailable",
		SpoolsTitle:           "Spools",
		AddSpool:              "Add spool",
		Manufacturer:          "Manufacturer",
		Material:              "Material",
		Color:                 "Color",
		ColorHex:              "Color HEX",
		DiameterMM:            "Diameter (mm)",
		WeightG:               "Weight (g)",
		Price:                 "Price",
		PurchaseDate:          "Purchase date",
		Save:                  "Save",
		Cancel:                "Cancel",
		Remaining:             "remaining",
		Archived:              "archived",
		Archive:               "Archive",
		PrintsTitle:           "Prints",
		AddPrint:              "Add print",
		SelectGCode:           "Select G-code file",
		SelectSpool:           "Select spool",
		Status:                "Status",
		StatusSuccess:         "Success",
		StatusFailed:          "Failed",
		StatusCancelled:       "Cancelled",
		CompletionPercent:     "Completion %",
		Notes:                 "Notes",
		PrintDate:             "Print date",
		EstimatedWeight:       "Estimated weight",
		ActualWeight:          "Actual weight",
		Duration:              "Duration",
		NozzleTemp:            "Nozzle temp",
		BedTemp:               "Bed temp",
		Slicer:                "Slicer",
		DryingTitle:           "Drying",
		DryingRecommendations: "Drying recommendations:",
		DryingLog:             "Drying log:",
		AddDrying:             "Add drying",
		TemperatureC:          "Temperature (°C)",
		DurationHours:         "Duration (hours)",
		MaxDaysNoDry:          "Max days without drying",
		SettingsTitle:         "Settings",
		LanguageLabel:         "Language",
		DryingProfiles:        "Drying profiles",
		CustomMaterials:       "Custom materials",
		AddMaterial:           "Add material",
		DeleteMaterial:        "Delete",
		MaterialName:          "Material name",
		SaveSettings:          "Save",
		BuiltInMaterial:       "(built-in)",
		DryingFrequency:       "Drying frequency",
		Days:                  "days",
		Hours:                 "h",
		Edit:                  "Edit",
		Delete:                "Delete",
		Add:                   "Add",
		Close:                 "Close",
		ErrorPrefix:           "Error",
		UsageFormat:           "Usage: %.1f g | Completion: %d%% | %s",
		NoPrintRecords:        "No print records.",
		GCodeFilePath:         "Path to G-code file",
		LoadGCode:             "Load G-code",
		SpoolColon:            "Spool:",
		NoSpoolsAvail:         "No spools",
		StatusColon:           "Status:",
		CompletionPct:         "Completion (%):",
		EnterFilePath:         "Enter file path",
		ParseError:            "Parse error",
		SaveError:             "Save error",
		ModelInfo:             "✅ Model: %s | Slicer: %s | Weight: %.1f g | Length: %.0f mm | Time: %s",
		SpoolsScreenTitle:     "Spools",
		AddSpoolBtn:           "Add spool",
		ManufacturerPH:        "Manufacturer",
		ColorPH:               "Color",
		NetWeightPH:           "Net weight (g)",
		PricePH:               "Price",
		SpoolInfoFmt:          "%s %s (%s) — %.0f g / %.0f g (%.0f%%)",
		RestartNote:           "Language changes apply immediately.",
	},
	LangBG: {
		Dashboard:             "Табло",
		Spools:                "Макари",
		Prints:                "Печатове",
		Drying:                "Сушене",
		Settings:              "Настройки",
		DashboardTitle:        "Табло",
		TotalSpools:           "Общо макари",
		TotalPrints:           "Общо печатове",
		SuccessRate:           "Успеваемост",
		TotalUsed:             "Общо използвано",
		AvgPerPrint:           "Средно на печат",
		NoDryingWarnings:      "Няма предупреждения за сушене",
		SpoolNeedsDrying:      "нуждае се от сушене",
		DaysAgo:               "дни назад",
		NeverDried:            "никога не е сушена",
		DryingWarnings:        "Предупреждения за сушене",
		NoSpools:              "Няма макари",
		StatsUnavailable:      "Статистиката не е налична",
		SpoolsTitle:           "Макари",
		AddSpool:              "Добави макара",
		Manufacturer:          "Производител",
		Material:              "Материал",
		Color:                 "Цвят",
		ColorHex:              "Цвят HEX",
		DiameterMM:            "Диаметър (мм)",
		WeightG:               "Тегло (г)",
		Price:                 "Цена",
		PurchaseDate:          "Дата на покупка",
		Save:                  "Запази",
		Cancel:                "Отказ",
		Remaining:             "остатък",
		Archived:              "архивирана",
		Archive:               "Архивирай",
		PrintsTitle:           "Печатове",
		AddPrint:              "Добави печат",
		SelectGCode:           "Избери G-code файл",
		SelectSpool:           "Избери макара",
		Status:                "Статус",
		StatusSuccess:         "Успешен",
		StatusFailed:          "Неуспешен",
		StatusCancelled:       "Отменен",
		CompletionPercent:     "Завършеност %",
		Notes:                 "Бележки",
		PrintDate:             "Дата на печат",
		EstimatedWeight:       "Очаквано тегло",
		ActualWeight:          "Действително тегло",
		Duration:              "Продължителност",
		NozzleTemp:            "Темп. на дюзата",
		BedTemp:               "Темп. на леглото",
		Slicer:                "Слайсер",
		DryingTitle:           "Сушене",
		DryingRecommendations: "Препоръки за сушене:",
		DryingLog:             "Дневник на сушене:",
		AddDrying:             "Добави сушене",
		TemperatureC:          "Температура (°C)",
		DurationHours:         "Продължителност (часове)",
		MaxDaysNoDry:          "Макс. дни без сушене",
		SettingsTitle:         "Настройки",
		LanguageLabel:         "Език",
		DryingProfiles:        "Профили за сушене",
		CustomMaterials:       "Потребителски материали",
		AddMaterial:           "Добави материал",
		DeleteMaterial:        "Изтрий",
		MaterialName:          "Име на материал",
		SaveSettings:          "Запази",
		BuiltInMaterial:       "(вграден)",
		DryingFrequency:       "Честота на сушене",
		Days:                  "дни",
		Hours:                 "ч",
		Edit:                  "Редактирай",
		Delete:                "Изтрий",
		Add:                   "Добави",
		Close:                 "Затвори",
		ErrorPrefix:           "Грешка",
		UsageFormat:           "Разход: %.1f г | Завършеност: %d%% | %s",
		NoPrintRecords:        "Няма записи за печатове.",
		GCodeFilePath:         "Път до G-code файл",
		LoadGCode:             "Зареди G-code",
		SpoolColon:            "Макара:",
		NoSpoolsAvail:         "Няма макари",
		StatusColon:           "Статус:",
		CompletionPct:         "Завършеност (%):",
		EnterFilePath:         "Въведете път до файл",
		ParseError:            "Грешка при парсване",
		SaveError:             "Грешка при запазване",
		ModelInfo:             "✅ Модел: %s | Слайсер: %s | Тегло: %.1f г | Дължина: %.0f мм | Време: %s",
		SpoolsScreenTitle:     "Макари",
		AddSpoolBtn:           "Добави макара",
		ManufacturerPH:        "Производител",
		ColorPH:               "Цвят",
		NetWeightPH:           "Тегло нето (г)",
		PricePH:               "Цена",
		SpoolInfoFmt:          "%s %s (%s) — %.0f г / %.0f г (%.0f%%)",
		RestartNote:           "Промените в езика се прилагат веднага.",
	},
}

var (
	currentLang Language = LangEN
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
	}
}
