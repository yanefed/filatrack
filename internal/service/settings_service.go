package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"context"
	"fmt"
	"strconv"
	"strings"
)

type SettingsService struct {
	settings repository.SettingsRepository
}

const allowLowFilamentSettingKey = "allow_low_filament_warning"
const lowFilamentLevelSettingKey = "low_filament_level"

func NewSettingsService(settings repository.SettingsRepository) *SettingsService {
	return &SettingsService{settings: settings}
}

func (s *SettingsService) GetLanguage(ctx context.Context) i18n.Language {
	val, err := s.settings.GetSetting(ctx, "language")
	if err != nil {
		return i18n.LangEN
	}
	lang := i18n.Language(val)
	if !i18n.IsSupportedLanguage(lang) {
		return i18n.LangEN
	}
	return lang
}

func (s *SettingsService) SetLanguage(ctx context.Context, lang i18n.Language) error {
	if !i18n.IsSupportedLanguage(lang) {
		return fmt.Errorf("unsupported language: %s", lang)
	}
	i18n.SetLanguage(lang)
	return s.settings.SetSetting(ctx, "language", string(lang))
}

func (s *SettingsService) AllowLowFilamentWarning(ctx context.Context) bool {
	val, err := s.settings.GetSetting(ctx, allowLowFilamentSettingKey)
	if err != nil {
		return false
	}
	enabled, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return enabled
}

func (s *SettingsService) SetAllowLowFilamentWarning(ctx context.Context, enabled bool) error {
	return s.settings.SetSetting(ctx, allowLowFilamentSettingKey, strconv.FormatBool(enabled))
}

func (s *SettingsService) GetLowFilamentLevel(ctx context.Context) int {
	val, err := s.settings.GetSetting(ctx, lowFilamentLevelSettingKey)
	if err != nil {
		return 0
	}
	level, err := strconv.Atoi(val)
	if err != nil || level < 0 {
		return 0
	}
	return level
}

func (s *SettingsService) SetLowFilamentLevel(ctx context.Context, value int) error {
	if value < 0 {
		return fmt.Errorf("low filament level must not be negative")
	}
	return s.settings.SetSetting(ctx, lowFilamentLevelSettingKey, strconv.Itoa(value))
}

func (s *SettingsService) ListMaterials(ctx context.Context) ([]domain.MaterialDefinition, error) {
	return s.settings.ListMaterials(ctx)
}

func (s *SettingsService) AllMaterials(ctx context.Context) ([]domain.Material, error) {
	definitions, err := s.settings.ListMaterials(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Material, len(definitions))
	for i := range definitions {
		result[i] = definitions[i].Name
	}
	return result, nil
}

func (s *SettingsService) CreateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	if material == nil {
		return fmt.Errorf("material is required")
	}
	copy := *material
	copy.Name = domain.Material(strings.TrimSpace(string(copy.Name)))
	if err := copy.Validate(); err != nil {
		return err
	}
	return s.settings.CreateMaterial(ctx, &copy)
}

func (s *SettingsService) UpdateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	if err := material.Validate(); err != nil {
		return err
	}
	return s.settings.UpdateMaterial(ctx, material)
}

func (s *SettingsService) DeleteMaterial(ctx context.Context, name domain.Material) error {
	if strings.TrimSpace(string(name)) == "" {
		return fmt.Errorf("material name is required")
	}
	return s.settings.DeleteMaterial(ctx, name)
}
