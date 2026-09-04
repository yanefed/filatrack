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

func (s *SettingsService) ListDryingProfiles(ctx context.Context) ([]domain.DryingProfile, error) {
	return s.settings.ListDryingProfiles(ctx)
}

func (s *SettingsService) SaveDryingProfile(ctx context.Context, p *domain.DryingProfile) error {
	if err := p.ValidateDryingProfile(); err != nil {
		return err
	}
	return s.settings.UpsertDryingProfile(ctx, p)
}

func (s *SettingsService) AllMaterials(ctx context.Context) ([]domain.Material, error) {
	custom, err := s.settings.ListCustomMaterials(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Material, len(domain.BuiltInMaterials))
	copy(result, domain.BuiltInMaterials)
	for _, c := range custom {
		result = append(result, domain.Material(c))
	}
	return result, nil
}

func (s *SettingsService) AddCustomMaterial(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("material name is required")
	}
	material := domain.Material(name)
	if material.IsBuiltIn() {
		return fmt.Errorf("material %s is built-in", name)
	}
	return s.settings.AddCustomMaterial(ctx, name)
}

func (s *SettingsService) DeleteCustomMaterial(ctx context.Context, name string) error {
	material := domain.Material(name)
	if material.IsBuiltIn() {
		return fmt.Errorf("cannot delete built-in material %s", name)
	}
	return s.settings.DeleteCustomMaterial(ctx, name)
}

func (s *SettingsService) ListCustomMaterialBindings(ctx context.Context) ([]domain.CustomMaterial, error) {
	return s.settings.ListCustomMaterialBindings(ctx)
}

func (s *SettingsService) BindCustomMaterialDryingProfile(ctx context.Context, material, profileMaterial domain.Material) error {
	if material.IsBuiltIn() {
		return fmt.Errorf("material %s is built-in", material)
	}
	if profileMaterial == "" {
		return fmt.Errorf("drying profile is required")
	}
	profile, err := s.settings.GetDryingProfileForMaterial(ctx, profileMaterial)
	if err != nil {
		return err
	}
	if profile == nil {
		return fmt.Errorf("drying profile %s not found", profileMaterial)
	}
	return s.settings.SetCustomMaterialDryingProfile(ctx, string(material), string(profile.Material))
}

func (s *SettingsService) CreateCustomMaterialDryingProfile(ctx context.Context, material domain.Material, profile *domain.DryingProfile) error {
	if material.IsBuiltIn() {
		return fmt.Errorf("material %s is built-in", material)
	}
	if profile == nil {
		return fmt.Errorf("drying profile is required")
	}
	owned := *profile
	owned.Material = material
	if err := owned.ValidateDryingProfile(); err != nil {
		return err
	}
	return s.settings.UpsertCustomMaterialDryingProfile(ctx, string(material), &owned)
}

func (s *SettingsService) GetDryingProfile(ctx context.Context, material domain.Material) (*domain.DryingProfile, error) {
	return s.settings.GetDryingProfileForMaterial(ctx, material)
}
