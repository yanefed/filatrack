package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
)

type spoolRepoStub struct {
	created   *domain.Spool
	createErr error
	spool     *domain.Spool
	getErr    error
	list      []domain.Spool
	listErr   error
	used      float64
	usedErr   error
}

func (r *spoolRepoStub) Create(_ context.Context, spool *domain.Spool) error {
	r.created = spool
	return r.createErr
}
func (r *spoolRepoStub) GetByID(context.Context, string) (*domain.Spool, error) {
	return r.spool, r.getErr
}
func (r *spoolRepoStub) List(context.Context, repository.SpoolFilter) ([]domain.Spool, error) {
	return r.list, r.listErr
}
func (r *spoolRepoStub) Update(context.Context, *domain.Spool) error { return nil }
func (r *spoolRepoStub) Archive(context.Context, string) error       { return nil }
func (r *spoolRepoStub) GetUsedWeight(context.Context, string) (float64, error) {
	return r.used, r.usedErr
}

type printRepoStub struct {
	created   *domain.PrintJob
	createErr error
	list      []domain.PrintJob
	listErr   error
	stats     *repository.PrintStats
	statsErr  error
	spoolID   string
	jobID     string
	updateErr error
}

func (r *printRepoStub) Create(_ context.Context, job *domain.PrintJob) error {
	r.created = job
	return r.createErr
}
func (r *printRepoStub) GetByID(context.Context, string) (*domain.PrintJob, error) {
	return nil, nil
}
func (r *printRepoStub) List(context.Context, repository.PrintJobFilter) ([]domain.PrintJob, error) {
	return r.list, r.listErr
}
func (r *printRepoStub) GetStats(context.Context, repository.StatsFilter) (*repository.PrintStats, error) {
	return r.stats, r.statsErr
}
func (r *printRepoStub) UpdateSpool(_ context.Context, jobID, spoolID string) error {
	r.jobID, r.spoolID = jobID, spoolID
	return r.updateErr
}

type dryingRepoStub struct {
	created   *domain.DryingSession
	createErr error
	list      []domain.DryingSession
	listErr   error
	last      *domain.DryingSession
	lastErr   error
}

func (r *dryingRepoStub) Create(_ context.Context, session *domain.DryingSession) error {
	r.created = session
	return r.createErr
}
func (r *dryingRepoStub) List(context.Context, string) ([]domain.DryingSession, error) {
	return r.list, r.listErr
}
func (r *dryingRepoStub) GetLastDrying(context.Context, string) (*domain.DryingSession, error) {
	return r.last, r.lastErr
}

type settingsRepoStub struct {
	setting         string
	getSettingErr   error
	setKey          string
	setValue        string
	setSettingErr   error
	profiles        []domain.DryingProfile
	profilesErr     error
	upserted        *domain.DryingProfile
	upsertErr       error
	customMaterials []string
	customErr       error
	addedMaterial   string
	addErr          error
	deletedMaterial string
	deleteErr       error
}

func (r *settingsRepoStub) GetSetting(context.Context, string) (string, error) {
	return r.setting, r.getSettingErr
}
func (r *settingsRepoStub) SetSetting(_ context.Context, key, value string) error {
	r.setKey, r.setValue = key, value
	return r.setSettingErr
}
func (r *settingsRepoStub) ListDryingProfiles(context.Context) ([]domain.DryingProfile, error) {
	return r.profiles, r.profilesErr
}
func (r *settingsRepoStub) UpsertDryingProfile(_ context.Context, profile *domain.DryingProfile) error {
	r.upserted = profile
	return r.upsertErr
}
func (r *settingsRepoStub) DeleteDryingProfile(context.Context, string) error { return nil }
func (r *settingsRepoStub) ListCustomMaterials(context.Context) ([]string, error) {
	return r.customMaterials, r.customErr
}
func (r *settingsRepoStub) AddCustomMaterial(_ context.Context, name string) error {
	r.addedMaterial = name
	return r.addErr
}
func (r *settingsRepoStub) DeleteCustomMaterial(_ context.Context, name string) error {
	r.deletedMaterial = name
	return r.deleteErr
}

type printerRepoStub struct {
	created       *domain.Printer
	createErr     error
	printer       *domain.Printer
	getErr        error
	list          []domain.Printer
	listErr       error
	assignedID    string
	assignedSpool string
	assignErr     error
	status        *domain.Printer
	statusErr     error
	updated       *domain.Printer
	updateErr     error
}

type checkerStub struct {
	printer domain.Printer
	err     error
}

func (c *checkerStub) Check(_ context.Context, printer domain.Printer) error {
	c.printer = printer
	return c.err
}

func (r *printerRepoStub) Create(_ context.Context, printer *domain.Printer) error {
	r.created = printer
	return r.createErr
}
func (r *printerRepoStub) GetByID(context.Context, string) (*domain.Printer, error) {
	return r.printer, r.getErr
}
func (r *printerRepoStub) List(context.Context) ([]domain.Printer, error) {
	return r.list, r.listErr
}
func (r *printerRepoStub) Update(_ context.Context, printer *domain.Printer) error {
	r.updated = printer
	return r.updateErr
}
func (r *printerRepoStub) AssignSpool(_ context.Context, printerID, spoolID string) error {
	r.assignedID, r.assignedSpool = printerID, spoolID
	return r.assignErr
}
func (r *printerRepoStub) UpdateStatus(_ context.Context, printer *domain.Printer) error {
	r.status = printer
	return r.statusErr
}

func TestSpoolServiceCreateValidationAndDefaults(t *testing.T) {
	repo := &spoolRepoStub{}
	service := NewSpoolService(repo, &dryingRepoStub{}, nil)

	if err := service.Create(t.Context(), &domain.Spool{}); err == nil {
		t.Fatal("expected non-positive weight to be rejected")
	}
	if repo.created != nil {
		t.Fatal("repository must not be called for invalid spool")
	}

	spool := &domain.Spool{Material: domain.MaterialPETG, TotalWeightG: 1000}
	before := time.Now()
	if err := service.Create(t.Context(), spool); err != nil {
		t.Fatalf("creating spool: %v", err)
	}
	if spool.DiameterMM != 1.75 || spool.DensityGCM3 != domain.DefaultDensities[domain.MaterialPETG] {
		t.Fatalf("unexpected defaults: diameter=%v density=%v", spool.DiameterMM, spool.DensityGCM3)
	}
	if spool.CreatedAt.Before(before) || spool.UpdatedAt.Before(before) || !spool.CreatedAt.Equal(spool.UpdatedAt) {
		t.Fatalf("timestamps were not initialized together: %+v", spool)
	}
	if repo.created != spool {
		t.Fatal("expected spool to be forwarded to repository")
	}
}

func TestSpoolServiceCreateUnknownMaterialAndRepositoryError(t *testing.T) {
	wantErr := errors.New("create failed")
	repo := &spoolRepoStub{createErr: wantErr}
	service := NewSpoolService(repo, &dryingRepoStub{}, nil)
	spool := &domain.Spool{Material: "Custom", TotalWeightG: 500, DiameterMM: 2.85}

	if err := service.Create(t.Context(), spool); !errors.Is(err, wantErr) {
		t.Fatalf("Create error = %v, want %v", err, wantErr)
	}
	if spool.DiameterMM != 2.85 || spool.DensityGCM3 != 1.24 {
		t.Fatalf("unexpected custom material defaults: %+v", spool)
	}
}

func TestSpoolServiceGetWithRemainingAndList(t *testing.T) {
	wantSpool := &domain.Spool{ID: "spool", TotalWeightG: 100}
	repo := &spoolRepoStub{spool: wantSpool, used: 30, list: []domain.Spool{*wantSpool}}
	service := NewSpoolService(repo, &dryingRepoStub{}, nil)

	got, remaining, err := service.GetWithRemaining(t.Context(), "spool")
	if err != nil || got != wantSpool || remaining != 70 {
		t.Fatalf("GetWithRemaining() = (%+v, %v, %v)", got, remaining, err)
	}
	listed, err := service.List(t.Context(), repository.SpoolFilter{})
	if err != nil || len(listed) != 1 || listed[0].ID != "spool" {
		t.Fatalf("List() = (%+v, %v)", listed, err)
	}

	getErr := errors.New("get failed")
	repo.getErr = getErr
	if _, _, err := service.GetWithRemaining(t.Context(), "spool"); !errors.Is(err, getErr) {
		t.Fatalf("GetWithRemaining get error = %v", err)
	}
	repo.getErr = nil
	repo.usedErr = errors.New("sum failed")
	if _, _, err := service.GetWithRemaining(t.Context(), "spool"); !errors.Is(err, repo.usedErr) {
		t.Fatalf("GetWithRemaining sum error = %v", err)
	}
}

func TestSpoolServiceNeedsDrying(t *testing.T) {
	spool := &domain.Spool{ID: "spool", Material: domain.MaterialPLA}
	profile := domain.DryingProfile{Material: domain.MaterialPLA, MaxDaysNoDry: 3}
	dryings := &dryingRepoStub{}
	settings := &settingsRepoStub{profiles: []domain.DryingProfile{profile}}
	service := NewSpoolService(&spoolRepoStub{}, dryings, settings)

	need, err := service.NeedsDrying(t.Context(), spool)
	if err != nil || !need {
		t.Fatalf("never-dried spool NeedsDrying() = (%v, %v), want true", need, err)
	}
	dryings.last = &domain.DryingSession{DriedAt: time.Now()}
	need, err = service.NeedsDrying(t.Context(), spool)
	if err != nil || need {
		t.Fatalf("recently dried spool NeedsDrying() = (%v, %v), want false", need, err)
	}

	dryings.lastErr = errors.New("drying lookup failed")
	if _, err := service.NeedsDrying(t.Context(), spool); !errors.Is(err, dryings.lastErr) {
		t.Fatalf("NeedsDrying error = %v", err)
	}

	dryings.lastErr = nil
	dryings.last = nil
	settings.profilesErr = errors.New("settings unavailable")
	need, err = service.NeedsDrying(t.Context(), spool)
	if err != nil || !need {
		t.Fatalf("default-profile fallback = (%v, %v), want true", need, err)
	}
	dryings.last = &domain.DryingSession{DriedAt: time.Now()}
	need, err = service.NeedsDrying(t.Context(), spool)
	if err != nil || need {
		t.Fatalf("recent fallback-profile drying = (%v, %v), want false", need, err)
	}
	dryings.last = nil
	dryings.lastErr = errors.New("fallback drying lookup failed")
	if _, err := service.NeedsDrying(t.Context(), spool); !errors.Is(err, dryings.lastErr) {
		t.Fatalf("fallback NeedsDrying error = %v", err)
	}
	dryings.lastErr = nil

	unknown := &domain.Spool{ID: "custom", Material: "Custom"}
	need, err = service.NeedsDrying(t.Context(), unknown)
	if err != nil || need {
		t.Fatalf("unknown material NeedsDrying() = (%v, %v), want false", need, err)
	}

	nilSettingsService := NewSpoolService(&spoolRepoStub{}, dryings, nil)
	need, err = nilSettingsService.NeedsDrying(t.Context(), spool)
	if err != nil || !need {
		t.Fatalf("nil-settings fallback = (%v, %v), want true", need, err)
	}
}

func TestPrintServiceCreateFromGcode(t *testing.T) {
	spool := &domain.Spool{ID: "spool", DiameterMM: 1.75, DensityGCM3: 1.24}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{}
	service := NewPrintService(prints, spools)
	meta := &domain.GcodeMetadata{
		Filename: "part.gcode", ModelName: "part", FilamentWeightG: 20,
		FilamentLengthMM: 7000, PrintDuration: time.Hour, NozzleTempC: 210,
		BedTempC: 60, Slicer: "PrusaSlicer",
	}

	job, err := service.CreateFromGcode(t.Context(), spool.ID, meta, domain.PrintStatusSuccess, 25)
	if err != nil {
		t.Fatalf("CreateFromGcode: %v", err)
	}
	if job != prints.created || job.CompletionPercent != 100 || job.ActualWeightG != 20 {
		t.Fatalf("unexpected successful print job: %+v", job)
	}
	if job.ModelName != meta.ModelName || job.PrintDuration != meta.PrintDuration || job.NozzleTempC != 210 || job.BedTempC != 60 {
		t.Fatalf("metadata was not copied: %+v", job)
	}
}

func TestPrintServiceDerivedWeightAndErrors(t *testing.T) {
	spool := &domain.Spool{ID: "spool", DiameterMM: 1.75, DensityGCM3: 1.24}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{}
	service := NewPrintService(prints, spools)
	meta := &domain.GcodeMetadata{FilamentLengthMM: 1000}

	job, err := service.CreateFromGcode(t.Context(), spool.ID, meta, domain.PrintStatusFailed, 40)
	if err != nil {
		t.Fatalf("CreateFromGcode: %v", err)
	}
	if job.EstimatedWeightG <= 0 || math.Abs(job.ActualWeightG-job.EstimatedWeightG*0.4) > 1e-9 {
		t.Fatalf("unexpected derived weights: %+v", job)
	}

	spools.getErr = errors.New("spool missing")
	if _, err := service.CreateFromGcode(t.Context(), spool.ID, meta, domain.PrintStatusFailed, 40); !errors.Is(err, spools.getErr) {
		t.Fatalf("spool lookup error = %v", err)
	}
	spools.getErr = nil
	prints.createErr = errors.New("insert failed")
	if _, err := service.CreateFromGcode(t.Context(), spool.ID, meta, domain.PrintStatusFailed, 40); !errors.Is(err, prints.createErr) {
		t.Fatalf("print creation error = %v", err)
	}
}

func TestPrintServiceListAndStats(t *testing.T) {
	wantErr := errors.New("repository error")
	prints := &printRepoStub{
		list:     []domain.PrintJob{{ID: "job"}},
		stats:    &repository.PrintStats{TotalPrints: 1},
		listErr:  wantErr,
		statsErr: wantErr,
	}
	service := NewPrintService(prints, &spoolRepoStub{})

	if jobs, err := service.List(t.Context(), repository.PrintJobFilter{}); len(jobs) != 1 || !errors.Is(err, wantErr) {
		t.Fatalf("List() = (%+v, %v)", jobs, err)
	}
	if stats, err := service.GetStats(t.Context(), repository.StatsFilter{}); stats.TotalPrints != 1 || !errors.Is(err, wantErr) {
		t.Fatalf("GetStats() = (%+v, %v)", stats, err)
	}
}

func TestPrintServiceChangeSpool(t *testing.T) {
	spool := &domain.Spool{ID: "spool-2"}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{}
	service := NewPrintService(prints, spools)

	if err := service.ChangeSpool(t.Context(), "job-1", spool.ID); err != nil {
		t.Fatalf("ChangeSpool: %v", err)
	}
	if prints.jobID != "job-1" || prints.spoolID != spool.ID {
		t.Fatalf("repository update = job %q spool %q", prints.jobID, prints.spoolID)
	}

	spools.getErr = errors.New("spool missing")
	if err := service.ChangeSpool(t.Context(), "job-1", "missing"); !errors.Is(err, spools.getErr) {
		t.Fatalf("spool lookup error = %v", err)
	}
	spools.getErr = nil
	prints.updateErr = errors.New("update failed")
	if err := service.ChangeSpool(t.Context(), "job-1", spool.ID); !errors.Is(err, prints.updateErr) {
		t.Fatalf("update error = %v", err)
	}
}

func TestPrinterServiceCreateAssignStatusAndList(t *testing.T) {
	printers := &printerRepoStub{list: []domain.Printer{{ID: "printer-1"}}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1"}}
	service := NewPrinterService(printers, spools, &printRepoStub{})

	if err := service.CreateMoonrakerPrinter(t.Context(), &domain.Printer{Name: "   "}); err == nil {
		t.Fatal("expected blank printer name error")
	}
	if err := service.CreateMoonrakerPrinter(t.Context(), &domain.Printer{Name: "Voron", MoonrakerURL: "ftp://printer"}); err == nil {
		t.Fatal("expected invalid Moonraker URL error")
	}

	printer := &domain.Printer{Name: " Voron ", MoonrakerURL: "http://printer.local/"}
	if err := service.CreateMoonrakerPrinter(t.Context(), printer); err != nil {
		t.Fatalf("CreateMoonrakerPrinter: %v", err)
	}
	if printers.created != printer || printer.Name != "Voron" || printer.MoonrakerURL != "http://printer.local" || printer.Kind != domain.PrinterKindMoonraker || !printer.Enabled || printer.CreatedAt.IsZero() {
		t.Fatalf("unexpected created printer: %+v", printer)
	}

	listed, err := service.List(t.Context())
	if err != nil || len(listed) != 1 {
		t.Fatalf("List() = (%+v, %v)", listed, err)
	}

	if err := service.AssignSpool(t.Context(), "printer-1", "spool-1"); err != nil {
		t.Fatalf("AssignSpool: %v", err)
	}
	if printers.assignedID != "printer-1" || printers.assignedSpool != "spool-1" {
		t.Fatalf("assignment = printer %q spool %q", printers.assignedID, printers.assignedSpool)
	}
	if err := service.ClearAssignedSpool(t.Context(), "printer-1"); err != nil {
		t.Fatalf("ClearAssignedSpool: %v", err)
	}
	if printers.assignedSpool != "" {
		t.Fatalf("expected cleared spool, got %q", printers.assignedSpool)
	}

	status := &domain.Printer{ID: "printer-1", Progress: 1.5}
	if err := service.UpdateStatus(t.Context(), status); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if printers.status != status || status.State != domain.PrinterStateUnknown || status.Progress != 1 || status.LastSeenAt.IsZero() {
		t.Fatalf("normalized status = %+v", status)
	}
}

func TestPrinterServiceCheckMoonrakerConnection(t *testing.T) {
	checker := &checkerStub{}
	service := NewPrinterService(&printerRepoStub{}, &spoolRepoStub{}, &printRepoStub{})
	service.SetConnectionChecker(checker)

	printer := &domain.Printer{MoonrakerURL: "http://printer.local/"}
	if err := service.CheckMoonrakerConnection(t.Context(), printer); err != nil {
		t.Fatalf("CheckMoonrakerConnection: %v", err)
	}
	if checker.printer.MoonrakerURL != "http://printer.local" || checker.printer.Name != "Moonraker" {
		t.Fatalf("normalized checked printer = %+v", checker.printer)
	}

	if err := service.CheckMoonrakerConnection(t.Context(), &domain.Printer{MoonrakerURL: "ftp://printer"}); err == nil {
		t.Fatal("expected invalid URL error")
	}
	checker.err = errors.New("connection failed")
	if err := service.CheckMoonrakerConnection(t.Context(), printer); !errors.Is(err, checker.err) {
		t.Fatalf("checker error = %v", err)
	}
}

func TestPrinterServiceRecordPrinterPrint(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{}
	service := NewPrinterService(printers, spools, prints)

	event := domain.PrinterPrintEvent{
		PrinterID:         "printer-1",
		ExternalPrintID:   "moonraker-123",
		GcodeFilename:     "folder/benchy.gcode",
		EstimatedLengthMM: 9000,
		PrintDuration:     time.Hour,
	}
	job, err := service.RecordPrinterPrint(t.Context(), event)
	if err != nil {
		t.Fatalf("RecordPrinterPrint: %v", err)
	}
	if job != prints.created || job.SpoolID != "spool-1" || job.PrinterID != "printer-1" || job.ExternalPrintID != "moonraker-123" || job.ModelName != "benchy" || job.Status != domain.PrintStatusSuccess || job.EstimatedWeightG <= 0 || job.ActualWeightG != job.EstimatedWeightG {
		t.Fatalf("unexpected printer print job: %+v", job)
	}

	printers.printer.ActiveSpoolID = ""
	if _, err := service.RecordPrinterPrint(t.Context(), event); err == nil || !strings.Contains(err.Error(), "no active spool") {
		t.Fatalf("expected active spool error, got %v", err)
	}
	printers.printer.ActiveSpoolID = "spool-1"
	prints.createErr = errors.New("insert failed")
	if _, err := service.RecordPrinterPrint(t.Context(), event); !errors.Is(err, prints.createErr) {
		t.Fatalf("create error = %v", err)
	}
}

func TestDryingServiceCreate(t *testing.T) {
	repo := &dryingRepoStub{}
	service := NewDryingService(repo, nil)
	if err := service.Create(t.Context(), &domain.DryingSession{DurationHours: 1}); err == nil {
		t.Fatal("expected invalid temperature error")
	}
	if err := service.Create(t.Context(), &domain.DryingSession{TemperatureC: 50}); err == nil {
		t.Fatal("expected invalid duration error")
	}

	session := &domain.DryingSession{TemperatureC: 50, DurationHours: 4}
	before := time.Now()
	if err := service.Create(t.Context(), session); err != nil {
		t.Fatalf("creating drying session: %v", err)
	}
	if session.DriedAt.Before(before) || session.CreatedAt.Before(before) || repo.created != session {
		t.Fatalf("drying timestamps/repository call are invalid: %+v", session)
	}

	preserved := time.Now().Add(-time.Hour)
	repo.createErr = errors.New("insert failed")
	session = &domain.DryingSession{TemperatureC: 50, DurationHours: 4, DriedAt: preserved}
	if err := service.Create(t.Context(), session); !errors.Is(err, repo.createErr) {
		t.Fatalf("Create error = %v", err)
	}
	if !session.DriedAt.Equal(preserved) {
		t.Fatal("explicit drying time must be preserved")
	}
}

func TestDryingServiceListAndRecommendations(t *testing.T) {
	wantErr := errors.New("list failed")
	dryings := &dryingRepoStub{list: []domain.DryingSession{{ID: "dry"}}, listErr: wantErr}
	custom := domain.DryingProfile{Material: domain.MaterialPLA, TemperatureC: 99, DurationHours: 1, MaxDaysNoDry: 1}
	settings := &settingsRepoStub{profiles: []domain.DryingProfile{custom}}
	service := NewDryingService(dryings, settings)

	if sessions, err := service.ListForSpool(t.Context(), "spool"); len(sessions) != 1 || !errors.Is(err, wantErr) {
		t.Fatalf("ListForSpool() = (%+v, %v)", sessions, err)
	}
	if got := service.GetRecommendation(t.Context(), domain.MaterialPLA); got == nil || got.TemperatureC != 99 {
		t.Fatalf("custom recommendation = %+v", got)
	}
	settings.profilesErr = errors.New("settings failed")
	if got := service.GetRecommendation(t.Context(), domain.MaterialPETG); got == nil || got.Material != domain.MaterialPETG {
		t.Fatalf("fallback recommendation = %+v", got)
	}
	if got := service.GetRecommendation(t.Context(), "Custom"); got != nil {
		t.Fatalf("unknown recommendation = %+v, want nil", got)
	}
	if got := NewDryingService(dryings, nil).GetRecommendation(t.Context(), domain.MaterialPLA); got == nil {
		t.Fatal("expected default recommendation with nil settings")
	}
}

func TestSettingsServiceLanguage(t *testing.T) {
	original := i18n.GetLanguage()
	t.Cleanup(func() { i18n.SetLanguage(original) })
	repo := &settingsRepoStub{setting: string(i18n.LangBG)}
	service := NewSettingsService(repo)

	if got := service.GetLanguage(t.Context()); got != i18n.LangBG {
		t.Fatalf("GetLanguage() = %q", got)
	}
	repo.setting = "invalid"
	if got := service.GetLanguage(t.Context()); got != i18n.LangEN {
		t.Fatalf("invalid language fallback = %q", got)
	}
	repo.getSettingErr = errors.New("read failed")
	if got := service.GetLanguage(t.Context()); got != i18n.LangEN {
		t.Fatalf("read-error language fallback = %q", got)
	}

	if err := service.SetLanguage(t.Context(), "de"); err == nil {
		t.Fatal("expected unsupported language error")
	}
	repo.setSettingErr = errors.New("write failed")
	if err := service.SetLanguage(t.Context(), i18n.LangBG); !errors.Is(err, repo.setSettingErr) {
		t.Fatalf("SetLanguage error = %v", err)
	}
	if repo.setKey != "language" || repo.setValue != string(i18n.LangBG) || i18n.GetLanguage() != i18n.LangBG {
		t.Fatalf("language was not applied and persisted: key=%q value=%q current=%q", repo.setKey, repo.setValue, i18n.GetLanguage())
	}
}

func TestSettingsServiceDryingProfiles(t *testing.T) {
	wantErr := errors.New("repository error")
	repo := &settingsRepoStub{profiles: []domain.DryingProfile{{Material: domain.MaterialPLA}}, profilesErr: wantErr}
	service := NewSettingsService(repo)
	if profiles, err := service.ListDryingProfiles(t.Context()); len(profiles) != 1 || !errors.Is(err, wantErr) {
		t.Fatalf("ListDryingProfiles() = (%+v, %v)", profiles, err)
	}

	invalid := []domain.DryingProfile{
		{},
		{Material: domain.MaterialPLA},
		{Material: domain.MaterialPLA, TemperatureC: 50},
		{Material: domain.MaterialPLA, TemperatureC: 50, DurationHours: 4},
	}
	for _, profile := range invalid {
		if err := service.SaveDryingProfile(t.Context(), &profile); err == nil {
			t.Fatalf("expected validation error for %+v", profile)
		}
	}
	valid := &domain.DryingProfile{Material: domain.MaterialPLA, TemperatureC: 50, DurationHours: 4, MaxDaysNoDry: 7}
	repo.upsertErr = wantErr
	if err := service.SaveDryingProfile(t.Context(), valid); !errors.Is(err, wantErr) || repo.upserted != valid {
		t.Fatalf("SaveDryingProfile() = %v, upserted=%+v", err, repo.upserted)
	}

	repo.profilesErr = nil
	repo.profiles = []domain.DryingProfile{{Material: domain.MaterialPETG}}
	if profile, err := service.GetDryingProfile(t.Context(), domain.MaterialPETG); err != nil || profile == nil {
		t.Fatalf("GetDryingProfile found = (%+v, %v)", profile, err)
	}
	if profile, err := service.GetDryingProfile(t.Context(), domain.MaterialPLA); err != nil || profile != nil {
		t.Fatalf("GetDryingProfile missing = (%+v, %v)", profile, err)
	}
	repo.profilesErr = wantErr
	if _, err := service.GetDryingProfile(t.Context(), domain.MaterialPLA); !errors.Is(err, wantErr) {
		t.Fatalf("GetDryingProfile error = %v", err)
	}
}

func TestSettingsServiceCustomMaterials(t *testing.T) {
	wantErr := errors.New("repository error")
	repo := &settingsRepoStub{customMaterials: []string{"Wood"}}
	service := NewSettingsService(repo)

	materials, err := service.AllMaterials(t.Context())
	if err != nil || len(materials) != len(BuiltInMaterials)+1 || materials[len(materials)-1] != "Wood" {
		t.Fatalf("AllMaterials() = (%+v, %v)", materials, err)
	}
	repo.customErr = wantErr
	if _, err := service.AllMaterials(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("AllMaterials error = %v", err)
	}

	if err := service.AddCustomMaterial(t.Context(), "   "); err == nil {
		t.Fatal("expected blank material error")
	}
	if err := service.AddCustomMaterial(t.Context(), "PLA"); err == nil {
		t.Fatal("expected built-in material error")
	}
	repo.addErr = wantErr
	if err := service.AddCustomMaterial(t.Context(), "  Wood  "); !errors.Is(err, wantErr) || repo.addedMaterial != "Wood" {
		t.Fatalf("AddCustomMaterial() = %v, added=%q", err, repo.addedMaterial)
	}

	if err := service.DeleteCustomMaterial(t.Context(), "PETG"); err == nil {
		t.Fatal("expected built-in deletion error")
	}
	repo.deleteErr = wantErr
	if err := service.DeleteCustomMaterial(t.Context(), "Wood"); !errors.Is(err, wantErr) || repo.deletedMaterial != "Wood" {
		t.Fatalf("DeleteCustomMaterial() = %v, deleted=%q", err, repo.deletedMaterial)
	}
	if !service.IsBuiltIn(domain.MaterialABS) || service.IsBuiltIn("Wood") {
		t.Fatal("IsBuiltIn returned unexpected result")
	}
	if !strings.Contains(service.DeleteCustomMaterial(t.Context(), "PLA").Error(), "built-in") {
		t.Fatal("built-in deletion error should be descriptive")
	}
}
