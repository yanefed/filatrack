package database

import (
	"context"

	"github.com/yanefed/filatrack/internal/domain"
)

// Stable repository handles route to the active backend under the state lock,
// allowing services to keep using them after reconnecting from the offline cache.
func (m *Manager) wrapRepositories() {
	m.repositories = &backend{
		spools:        &cachedSpoolRepository{manager: m},
		prints:        &cachedPrintJobRepository{manager: m},
		dryings:       &cachedDryingRepository{manager: m},
		settings:      &cachedSettingsRepository{manager: m},
		printers:      &cachedPrinterRepository{manager: m},
		models:        &cachedPrinterModelRepository{manager: m},
		previewImages: &cachedPreviewImageRepository{manager: m},
	}
}

type cachedSpoolRepository struct{ manager *Manager }

func (r *cachedSpoolRepository) Create(ctx context.Context, spool *domain.Spool) error {
	return r.manager.write(func() error { return r.manager.active.spools.Create(ctx, spool) })
}

func (r *cachedSpoolRepository) Update(ctx context.Context, spool *domain.Spool) error {
	return r.manager.write(func() error { return r.manager.active.spools.Update(ctx, spool) })
}

func (r *cachedSpoolRepository) Archive(ctx context.Context, id string) error {
	return r.manager.write(func() error { return r.manager.active.spools.Archive(ctx, id) })
}

func (r *cachedSpoolRepository) GetByID(ctx context.Context, id string) (*domain.Spool, error) {
	return readRepository(r.manager, func(b *backend) (*domain.Spool, error) { return b.spools.GetByID(ctx, id) })
}

func (r *cachedSpoolRepository) List(ctx context.Context, filter domain.SpoolFilter) ([]domain.Spool, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.Spool, error) { return b.spools.List(ctx, filter) })
}

func (r *cachedSpoolRepository) GetUsedWeight(ctx context.Context, spoolID string) (float64, error) {
	return readRepository(r.manager, func(b *backend) (float64, error) { return b.spools.GetUsedWeight(ctx, spoolID) })
}

type cachedPrintJobRepository struct{ manager *Manager }

func (r *cachedPrintJobRepository) Create(ctx context.Context, job *domain.PrintJob) error {
	return r.manager.write(func() error { return r.manager.active.prints.Create(ctx, job) })
}

func (r *cachedPrintJobRepository) UpdateSpool(ctx context.Context, jobID, spoolID string) error {
	return r.manager.write(func() error { return r.manager.active.prints.UpdateSpool(ctx, jobID, spoolID) })
}

func (r *cachedPrintJobRepository) UpdateUsage(ctx context.Context, jobID string, estimatedWeightG, actualWeightG, estimatedLengthMM float64, completionPercent int, usageMeasured bool) error {
	return r.manager.write(func() error {
		return r.manager.active.prints.UpdateUsage(ctx, jobID, estimatedWeightG, actualWeightG, estimatedLengthMM, completionPercent, usageMeasured)
	})
}

func (r *cachedPrintJobRepository) GetByID(ctx context.Context, id string) (*domain.PrintJob, error) {
	return readRepository(r.manager, func(b *backend) (*domain.PrintJob, error) { return b.prints.GetByID(ctx, id) })
}

func (r *cachedPrintJobRepository) List(ctx context.Context, filter domain.PrintJobFilter) ([]domain.PrintJob, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.PrintJob, error) { return b.prints.List(ctx, filter) })
}

func (r *cachedPrintJobRepository) GetStats(ctx context.Context, filter domain.StatsFilter) (*domain.PrintStats, error) {
	return readRepository(r.manager, func(b *backend) (*domain.PrintStats, error) { return b.prints.GetStats(ctx, filter) })
}

type cachedDryingRepository struct{ manager *Manager }

func (r *cachedDryingRepository) Create(ctx context.Context, session *domain.DryingSession) error {
	return r.manager.write(func() error { return r.manager.active.dryings.Create(ctx, session) })
}

func (r *cachedDryingRepository) List(ctx context.Context, spoolID string) ([]domain.DryingSession, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.DryingSession, error) { return b.dryings.List(ctx, spoolID) })
}

func (r *cachedDryingRepository) GetLastDrying(ctx context.Context, spoolID string) (*domain.DryingSession, error) {
	return readRepository(r.manager, func(b *backend) (*domain.DryingSession, error) { return b.dryings.GetLastDrying(ctx, spoolID) })
}

type cachedSettingsRepository struct{ manager *Manager }

func (r *cachedSettingsRepository) SetSetting(ctx context.Context, key, value string) error {
	return r.manager.write(func() error { return r.manager.active.settings.SetSetting(ctx, key, value) })
}

func (r *cachedSettingsRepository) CreateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	return r.manager.write(func() error { return r.manager.active.settings.CreateMaterial(ctx, material) })
}

func (r *cachedSettingsRepository) UpdateMaterial(ctx context.Context, material *domain.MaterialDefinition) error {
	return r.manager.write(func() error { return r.manager.active.settings.UpdateMaterial(ctx, material) })
}

func (r *cachedSettingsRepository) DeleteMaterial(ctx context.Context, name domain.Material) error {
	return r.manager.write(func() error { return r.manager.active.settings.DeleteMaterial(ctx, name) })
}

func (r *cachedSettingsRepository) GetSetting(ctx context.Context, key string) (string, error) {
	return readRepository(r.manager, func(b *backend) (string, error) { return b.settings.GetSetting(ctx, key) })
}

func (r *cachedSettingsRepository) ListMaterials(ctx context.Context) ([]domain.MaterialDefinition, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.MaterialDefinition, error) { return b.settings.ListMaterials(ctx) })
}

func (r *cachedSettingsRepository) GetMaterial(ctx context.Context, name domain.Material) (*domain.MaterialDefinition, error) {
	return readRepository(r.manager, func(b *backend) (*domain.MaterialDefinition, error) { return b.settings.GetMaterial(ctx, name) })
}

func (r *cachedSettingsRepository) GetDryingProfileForMaterial(ctx context.Context, material domain.Material) (*domain.DryingProfile, error) {
	return readRepository(r.manager, func(b *backend) (*domain.DryingProfile, error) {
		return b.settings.GetDryingProfileForMaterial(ctx, material)
	})
}

type cachedPrinterRepository struct{ manager *Manager }

func (r *cachedPrinterRepository) Create(ctx context.Context, printer *domain.Printer) error {
	return r.manager.write(func() error { return r.manager.active.printers.Create(ctx, printer) })
}

func (r *cachedPrinterRepository) Update(ctx context.Context, printer *domain.Printer) error {
	return r.manager.write(func() error { return r.manager.active.printers.Update(ctx, printer) })
}

func (r *cachedPrinterRepository) AssignSpool(ctx context.Context, printerID, spoolID string) error {
	return r.manager.write(func() error { return r.manager.active.printers.AssignSpool(ctx, printerID, spoolID) })
}

func (r *cachedPrinterRepository) UpdateStatus(ctx context.Context, printer *domain.Printer) error {
	return r.manager.write(func() error { return r.manager.active.printers.UpdateStatus(ctx, printer) })
}

func (r *cachedPrinterRepository) GetByID(ctx context.Context, id string) (*domain.Printer, error) {
	return readRepository(r.manager, func(b *backend) (*domain.Printer, error) { return b.printers.GetByID(ctx, id) })
}

func (r *cachedPrinterRepository) List(ctx context.Context) ([]domain.Printer, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.Printer, error) { return b.printers.List(ctx) })
}

type cachedPrinterModelRepository struct{ manager *Manager }

func (r *cachedPrinterModelRepository) SaveForPrinter(ctx context.Context, printerID string, models []domain.PrinterModelFile) error {
	return r.manager.write(func() error { return r.manager.active.models.SaveForPrinter(ctx, printerID, models) })
}

func (r *cachedPrinterModelRepository) List(ctx context.Context, printerID *string) ([]domain.PrinterModelFile, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.PrinterModelFile, error) { return b.models.List(ctx, printerID) })
}

type cachedPreviewImageRepository struct{ manager *Manager }

func (r *cachedPreviewImageRepository) Get(ctx context.Context, hash string) ([]byte, error) {
	return readRepository(r.manager, func(b *backend) ([]byte, error) { return b.previewImages.Get(ctx, hash) })
}

func (r *cachedPrintJobRepository) UpdatePreviewImageHash(ctx context.Context, jobID, hash string) error {
	return r.manager.write(func() error {
		return r.manager.active.prints.UpdatePreviewImageHash(ctx, jobID, hash)
	})
}

func (r *cachedPreviewImageRepository) Put(ctx context.Context, data []byte) (string, error) {
	var hash string
	err := r.manager.write(func() error {
		var err error
		hash, err = r.manager.active.previewImages.Put(ctx, data)
		return err
	})
	return hash, err
}

func (r *cachedSpoolRepository) GetUsedWeights(ctx context.Context) (map[string]float64, error) {
	return readRepository(r.manager, func(b *backend) (map[string]float64, error) { return b.spools.GetUsedWeights(ctx) })
}
func (r *cachedDryingRepository) Latest(ctx context.Context) (map[string]domain.DryingSession, error) {
	return readRepository(r.manager, func(b *backend) (map[string]domain.DryingSession, error) { return b.dryings.Latest(ctx) })
}
func (r *cachedDryingRepository) ListAll(ctx context.Context) ([]domain.DryingSession, error) {
	return readRepository(r.manager, func(b *backend) ([]domain.DryingSession, error) { return b.dryings.ListAll(ctx) })
}

func (r *cachedSpoolRepository) ArchiveWithAssignments(ctx context.Context, id string, detachPrinterIDs []string) (receipt domain.SpoolArchive, err error) {
	err = r.manager.write(func() error {
		var e error
		receipt, e = r.manager.active.spools.ArchiveWithAssignments(ctx, id, detachPrinterIDs)
		return e
	})
	return
}
func (r *cachedSpoolRepository) Restore(ctx context.Context, receipt domain.SpoolArchive) (result domain.SpoolRestoreResult, err error) {
	err = r.manager.write(func() error { var e error; result, e = r.manager.active.spools.Restore(ctx, receipt); return e })
	return
}
