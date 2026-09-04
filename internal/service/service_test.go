package service

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/i18n"
	"awesomeProject/internal/repository"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type spoolRepoStub struct {
	created    *domain.Spool
	createErr  error
	spool      *domain.Spool
	getErr     error
	list       []domain.Spool
	listErr    error
	used       float64
	usedErr    error
	updated    *domain.Spool
	updateErr  error
	archived   string
	archiveErr error
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
func (r *spoolRepoStub) Update(_ context.Context, spool *domain.Spool) error {
	r.updated = spool
	return r.updateErr
}
func (r *spoolRepoStub) Archive(_ context.Context, id string) error {
	r.archived = id
	return r.archiveErr
}
func (r *spoolRepoStub) GetUsedWeight(context.Context, string) (float64, error) {
	return r.used, r.usedErr
}

type printRepoStub struct {
	created         *domain.PrintJob
	createdJobs     []*domain.PrintJob
	createErr       error
	list            []domain.PrintJob
	listErr         error
	stats           *repository.PrintStats
	statsErr        error
	spoolID         string
	jobID           string
	usageJobID      string
	estimatedWeight float64
	actualWeight    float64
	estimatedLength float64
	completion      int
	updateErr       error
}

func (r *printRepoStub) Create(_ context.Context, job *domain.PrintJob) error {
	r.created = job
	r.createdJobs = append(r.createdJobs, job)
	return r.createErr
}
func (r *printRepoStub) GetByID(context.Context, string) (*domain.PrintJob, error) {
	return r.created, nil
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
func (r *printRepoStub) UpdateUsage(_ context.Context, jobID string, estimatedWeightG, actualWeightG, estimatedLengthMM float64, completionPercent int) error {
	r.usageJobID = jobID
	r.estimatedWeight = estimatedWeightG
	r.actualWeight = actualWeightG
	r.estimatedLength = estimatedLengthMM
	r.completion = completionPercent
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
	customBindings  []domain.CustomMaterial
	boundMaterial   string
	boundProfile    string
	ownedProfile    *domain.DryingProfile
}

func (r *settingsRepoStub) GetSetting(context.Context, string) (string, error) {
	return r.setting, r.getSettingErr
}
func (r *settingsRepoStub) SetSetting(_ context.Context, key, value string) error {
	r.setKey, r.setValue = key, value
	return r.setSettingErr
}

func newTestPrinterService(
	printers repository.PrinterRepository,
	spools repository.SpoolRepository,
	prints repository.PrintJobRepository,
) *PrinterService {
	return NewPrinterService(printers, spools, prints, &settingsRepoStub{setting: "50"})
}

func (r *settingsRepoStub) ListDryingProfiles(context.Context) ([]domain.DryingProfile, error) {
	return r.profiles, r.profilesErr
}
func (r *settingsRepoStub) UpsertDryingProfile(_ context.Context, profile *domain.DryingProfile) error {
	r.upserted = profile
	return r.upsertErr
}
func (r *settingsRepoStub) DeleteDryingProfile(context.Context, string) error { return nil }
func (r *settingsRepoStub) GetDryingProfileForMaterial(_ context.Context, material domain.Material) (*domain.DryingProfile, error) {
	if r.profilesErr != nil {
		return nil, r.profilesErr
	}
	for i := range r.profiles {
		if r.profiles[i].Material == material {
			profile := r.profiles[i]
			return &profile, nil
		}
	}
	for _, binding := range r.customBindings {
		if binding.Name == material {
			for i := range r.profiles {
				if r.profiles[i].Material == binding.DryingProfileMaterial {
					profile := r.profiles[i]
					return &profile, nil
				}
			}
		}
	}
	return nil, nil
}
func (r *settingsRepoStub) ListCustomMaterials(context.Context) ([]string, error) {
	return r.customMaterials, r.customErr
}
func (r *settingsRepoStub) ListCustomMaterialBindings(context.Context) ([]domain.CustomMaterial, error) {
	return r.customBindings, r.customErr
}
func (r *settingsRepoStub) AddCustomMaterial(_ context.Context, name string) error {
	r.addedMaterial = name
	return r.addErr
}
func (r *settingsRepoStub) SetCustomMaterialDryingProfile(_ context.Context, name, profile string) error {
	r.boundMaterial, r.boundProfile = name, profile
	return r.upsertErr
}
func (r *settingsRepoStub) UpsertCustomMaterialDryingProfile(_ context.Context, name string, profile *domain.DryingProfile) error {
	r.boundMaterial, r.boundProfile, r.ownedProfile = name, name, profile
	return r.upsertErr
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
	getByIDFunc   func(id string) (*domain.Printer, error)
}

type checkerStub struct {
	printer domain.Printer
	err     error
}

type printerJobClientStub struct {
	history       []domain.PrinterHistoryJob
	historyErr    error
	historyPages  map[int][]domain.PrinterHistoryJob
	historyCount  int
	historyStarts []int
	models        []domain.PrinterModelFile
	modelsErr     error
	metadata      *domain.GcodeMetadata
	metadataErr   error
	startedFile   string
	startErr      error
	downloaded    string
	downloadErr   error
	downloadData  []byte
	uploadedPath  string
	uploadedLocal string
	uploadStarted bool
	uploadErr     error
}

type printerModelRepoStub struct {
	savedPrinterID string
	saved          []domain.PrinterModelFile
	list           []domain.PrinterModelFile
	listErr        error
	saveErr        error
}

func (c *printerJobClientStub) ListHistory(context.Context, domain.Printer, int) ([]domain.PrinterHistoryJob, error) {
	return c.history, c.historyErr
}

func (c *printerJobClientStub) ListHistoryPage(_ context.Context, _ domain.Printer, start, _ int) ([]domain.PrinterHistoryJob, int, error) {
	c.historyStarts = append(c.historyStarts, start)
	if c.historyErr != nil {
		return nil, 0, c.historyErr
	}
	if c.historyPages == nil {
		return c.history, len(c.history), nil
	}
	return c.historyPages[start], c.historyCount, nil
}

func (c *printerJobClientStub) ListModels(context.Context, domain.Printer) ([]domain.PrinterModelFile, error) {
	return c.models, c.modelsErr
}

func (c *printerJobClientStub) FetchMetadata(context.Context, domain.Printer, string) (*domain.GcodeMetadata, error) {
	return c.metadata, c.metadataErr
}

func (c *printerJobClientStub) StartFile(_ context.Context, _ domain.Printer, filename string) error {
	c.startedFile = filename
	return c.startErr
}

func (c *printerJobClientStub) DownloadFile(_ context.Context, _ domain.Printer, filename, localPath string) error {
	c.downloaded = filename + ">" + localPath
	if c.downloadErr != nil {
		return c.downloadErr
	}
	data := c.downloadData
	if data == nil {
		data = []byte("G28\n")
	}
	return os.WriteFile(localPath, data, 0o644)
}

func (c *printerJobClientStub) Upload(_ context.Context, _ domain.Printer, localPath string, startPrint bool) (domain.PrinterUploadResult, error) {
	c.uploadedLocal = localPath
	c.uploadStarted = startPrint
	return domain.PrinterUploadResult{Path: c.uploadedPath, PrintStarted: startPrint}, c.uploadErr
}

func (r *printerModelRepoStub) SaveForPrinter(_ context.Context, printerID string, models []domain.PrinterModelFile) error {
	r.savedPrinterID = printerID
	r.saved = append([]domain.PrinterModelFile(nil), models...)
	r.list = append([]domain.PrinterModelFile(nil), models...)
	return r.saveErr
}

func (r *printerModelRepoStub) List(_ context.Context, _ *string) ([]domain.PrinterModelFile, error) {
	return r.list, r.listErr
}

func (c *checkerStub) Check(_ context.Context, printer domain.Printer) error {
	c.printer = printer
	return c.err
}

func (r *printerRepoStub) Create(_ context.Context, printer *domain.Printer) error {
	r.created = printer
	return r.createErr
}
func (r *printerRepoStub) GetByID(_ context.Context, id string) (*domain.Printer, error) {
	if r.getByIDFunc != nil {
		return r.getByIDFunc(id)
	}
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

func TestSpoolServiceUpdateAndArchive(t *testing.T) {
	repo := &spoolRepoStub{}
	service := NewSpoolService(repo, &dryingRepoStub{}, nil)
	spool := &domain.Spool{ID: "spool-1", Material: domain.MaterialPLA, TotalWeightG: 1000}

	if err := service.Update(t.Context(), spool); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if repo.updated != spool || spool.DiameterMM != 1.75 || spool.DensityGCM3 != domain.DefaultDensities[domain.MaterialPLA] {
		t.Fatalf("updated spool = %+v repo=%+v", spool, repo.updated)
	}

	if err := service.Archive(t.Context(), spool.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if repo.archived != spool.ID {
		t.Fatalf("archived id = %q", repo.archived)
	}

	if err := service.Update(t.Context(), &domain.Spool{ID: "bad"}); err == nil {
		t.Fatal("expected invalid update error")
	}
	if err := service.Archive(t.Context(), ""); err == nil {
		t.Fatal("expected invalid archive error")
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
	settings.customBindings = []domain.CustomMaterial{{Name: "Custom", DryingProfileMaterial: domain.MaterialPLA}}
	customSpool := &domain.Spool{ID: "custom", Material: "Custom"}
	need, err = service.NeedsDrying(t.Context(), customSpool)
	if err != nil || !need {
		t.Fatalf("bound custom material NeedsDrying() = (%v, %v), want true", need, err)
	}
	settings.customBindings = nil
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
	stats, err := service.GetStats(t.Context(), repository.StatsFilter{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetStats() = (%+v, %v)", stats, err)
	}
	if stats == nil || stats.TotalPrints != 1 {
		t.Fatalf("GetStats() stats = %+v", stats)
	}
}

func TestPrintServiceChangeSpool(t *testing.T) {
	spool := &domain.Spool{ID: "spool-2", DiameterMM: 1.75, DensityGCM3: 1.24}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{created: &domain.PrintJob{ID: "job-1"}}
	service := NewPrintService(prints, spools)

	if err := service.ChangeSpool(t.Context(), "job-1", spool.ID); err != nil {
		t.Fatalf("ChangeSpool: %v", err)
	}
	if prints.jobID != "job-1" || prints.spoolID != spool.ID {
		t.Fatalf("repository update = job %q spool %q", prints.jobID, prints.spoolID)
	}

	if err := service.ChangeSpool(t.Context(), "job-1", ""); err != nil {
		t.Fatalf("clearing spool: %v", err)
	}
	if prints.jobID != "job-1" || prints.spoolID != "" {
		t.Fatalf("repository clear = job %q spool %q", prints.jobID, prints.spoolID)
	}

	prints.created = &domain.PrintJob{ID: "job-2", EstimatedLengthMM: 1000, EstimatedWeightG: 10}
	if err := service.ChangeSpool(t.Context(), "job-2", spool.ID); err != nil {
		t.Fatalf("ChangeSpool with usage recalculation: %v", err)
	}
	if prints.usageJobID != "job-2" || prints.actualWeight <= 0 || prints.completion <= 0 {
		t.Fatalf("usage recalculation = job %q estimated %.3f actual %.3f completion %d", prints.usageJobID, prints.estimatedWeight, prints.actualWeight, prints.completion)
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
	service := newTestPrinterService(printers, spools, &printRepoStub{})

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
	service := newTestPrinterService(&printerRepoStub{}, &spoolRepoStub{}, &printRepoStub{})
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

func TestPrinterServiceUpdateMoonrakerPrinter(t *testing.T) {
	printers := &printerRepoStub{}
	checker := &checkerStub{}
	service := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{})
	service.SetConnectionChecker(checker)

	printer := &domain.Printer{
		ID:            "printer-1",
		Name:          "  Voron  ",
		MoonrakerURL:  " http://printer.local/ ",
		APIKey:        "secret",
		Kind:          domain.PrinterKindMoonraker,
		Enabled:       true,
		ActiveSpoolID: "spool-1",
	}
	if err := service.UpdateMoonrakerPrinter(t.Context(), printer); err != nil {
		t.Fatalf("UpdateMoonrakerPrinter: %v", err)
	}
	if printer.Name != "Voron" || printer.MoonrakerURL != "http://printer.local" {
		t.Fatalf("normalized printer = %+v", printer)
	}
	if checker.printer.Name != "Voron" || checker.printer.MoonrakerURL != "http://printer.local" {
		t.Fatalf("checked printer = %+v", checker.printer)
	}
	if printers.updated != printer {
		t.Fatalf("updated printer = %+v, want %+v", printers.updated, printer)
	}

	printers.updated = nil
	checker.err = errors.New("connection failed")
	if err := service.UpdateMoonrakerPrinter(t.Context(), printer); !errors.Is(err, checker.err) {
		t.Fatalf("connection error = %v, want %v", err, checker.err)
	}
	if printers.updated != nil {
		t.Fatalf("printer was updated after failed connection check: %+v", printers.updated)
	}

	checker.err = nil
	if err := service.UpdateMoonrakerPrinter(t.Context(), &domain.Printer{Name: " ", MoonrakerURL: "http://printer.local"}); err == nil {
		t.Fatal("expected blank printer name error")
	}
	if err := service.UpdateMoonrakerPrinter(t.Context(), &domain.Printer{Name: "Voron", MoonrakerURL: "ftp://printer"}); err == nil {
		t.Fatal("expected invalid URL error")
	}
	if err := service.UpdateMoonrakerPrinter(t.Context(), nil); err == nil {
		t.Fatal("expected nil printer error")
	}

	printers.updateErr = errors.New("update failed")
	if err := service.UpdateMoonrakerPrinter(t.Context(), printer); !errors.Is(err, printers.updateErr) {
		t.Fatalf("repository error = %v, want %v", err, printers.updateErr)
	}
}

func TestPrinterServiceRecordPrinterPrint(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{}
	service := newTestPrinterService(printers, spools, prints)

	event := domain.PrinterPrintEvent{
		PrinterID:         "printer-1",
		ExternalPrintID:   "moonraker-123",
		GcodeFilename:     "folder/benchy.gcode",
		EstimatedLengthMM: 9000,
		PrintDuration:     time.Hour,
		PreviewImagePath:  "/tmp/benchy.png",
	}
	job, err := service.RecordPrinterPrint(t.Context(), event)
	if err != nil {
		t.Fatalf("RecordPrinterPrint: %v", err)
	}
	if job != prints.created || job.SpoolID != "spool-1" || job.PrinterID != "printer-1" || job.ExternalPrintID != "moonraker-123" || job.ModelName != "benchy" || job.Status != domain.PrintStatusSuccess || job.EstimatedWeightG <= 0 || job.ActualWeightG != job.EstimatedWeightG || job.PreviewImagePath != "/tmp/benchy.png" {
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

func TestPrinterServiceListCombinedHistoryMarksLocalJobs(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	prints := &printRepoStub{list: []domain.PrintJob{{ID: "local-1", SpoolID: "spool-1", PrinterID: "printer-1", ExternalPrintID: "remote-1", PreviewImagePath: "/tmp/preview.png"}}}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{
		{PrinterID: "printer-1", ExternalPrintID: "remote-1", GcodeFilename: "one.gcode"},
		{PrinterID: "printer-1", ExternalPrintID: "remote-2", GcodeFilename: "two.gcode"},
	}}
	service := newTestPrinterService(printers, &spoolRepoStub{}, prints)
	service.SetJobClient(client)

	history, err := service.ListCombinedHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("ListCombinedHistory: %v", err)
	}
	if len(history) != 2 || history[0].LocalPrintJobID != "local-1" || history[0].LocalSpoolID != "spool-1" || history[0].PreviewImagePath != "/tmp/preview.png" || history[1].LocalPrintJobID != "" {
		t.Fatalf("combined history = %+v", history)
	}
}

func TestPrinterServiceAssignHistoryJobSpoolCreatesPrintJob(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{
		PrinterID:        "printer-1",
		ExternalPrintID:  "remote-1",
		GcodeFilename:    "folder/benchy.gcode",
		Status:           domain.PrinterHistoryStatusCompleted,
		FilamentUsedMM:   1000,
		PrintDuration:    time.Minute,
		NozzleTempC:      215,
		BedTempC:         60,
		Slicer:           "OrcaSlicer",
		PreviewImagePath: "/tmp/benchy.png",
		EndedAt:          time.Unix(1000, 0),
	}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	job, err := service.AssignHistoryJobSpool(t.Context(), "printer-1", "remote-1", "spool-1")
	if err != nil {
		t.Fatalf("AssignHistoryJobSpool: %v", err)
	}
	if job != prints.created || job.SpoolID != "spool-1" || job.ExternalPrintID != "remote-1" || job.ModelName != "benchy" || job.ActualWeightG <= 0 || job.PreviewImagePath != "/tmp/benchy.png" {
		t.Fatalf("created job = %+v", job)
	}
}

func TestPrinterServiceListModelsEnrichesMetadata(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	client := &printerJobClientStub{
		models:   []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "folder/cube.gcode", Filename: "cube.gcode"}},
		metadata: &domain.GcodeMetadata{Filename: "folder/cube.gcode", FilamentWeightG: 12.5, PrintDuration: time.Minute, Slicer: "OrcaSlicer", PreviewImagePath: "/tmp/cube.png"},
	}
	service := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{})
	service.SetJobClient(client)

	models, err := service.ListModels(t.Context(), "printer-1")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].Path != "folder/cube.gcode" || models[0].EstimatedWeightG != 12.5 || models[0].PrintDuration != time.Minute || models[0].Slicer != "OrcaSlicer" || models[0].PreviewImagePath != "/tmp/cube.png" {
		t.Fatalf("models = %+v", models)
	}
}

func TestPrinterServiceRefreshModelsSavesCache(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	client := &printerJobClientStub{
		models:   []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "folder/cube.gcode", Filename: "cube.gcode"}},
		metadata: &domain.GcodeMetadata{Filename: "folder/cube.gcode", FilamentWeightG: 12.5, PrintDuration: time.Minute, Slicer: "OrcaSlicer", PreviewImagePath: "/tmp/cube.png"},
	}
	cache := &printerModelRepoStub{
		list: []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "folder/cube.gcode", Filename: "cube.gcode", EstimatedWeightG: 12.5, PreviewImagePath: "/tmp/cube.png"}},
	}
	service := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{})
	service.SetJobClient(client)
	service.SetModelCache(cache)

	models, err := service.RefreshModels(t.Context(), "printer-1")
	if err != nil {
		t.Fatalf("RefreshModels: %v", err)
	}
	if cache.savedPrinterID != "printer-1" || len(cache.saved) != 1 || cache.saved[0].EstimatedWeightG != 12.5 || cache.saved[0].PreviewImagePath != "/tmp/cube.png" {
		t.Fatalf("saved cache printer=%q models=%+v", cache.savedPrinterID, cache.saved)
	}
	if len(models) != 1 || models[0].Path != "folder/cube.gcode" {
		t.Fatalf("models = %+v", models)
	}
}

func TestPrinterServiceRefreshModelsCalculatesWeightFromLength(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.25}}
	client := &printerJobClientStub{
		models:   []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "from-app.gcode", Filename: "from-app.gcode"}},
		metadata: &domain.GcodeMetadata{Filename: "from-app.gcode", FilamentLengthMM: 6931.39},
	}
	service := newTestPrinterService(printers, spools, &printRepoStub{})
	service.SetJobClient(client)

	models, err := service.RefreshModels(t.Context(), "printer-1")
	if err != nil {
		t.Fatalf("RefreshModels: %v", err)
	}
	want := domain.WeightFromLength(6931.39, 1.75, 1.25)
	if len(models) != 1 || math.Abs(models[0].EstimatedWeightG-want) > 0.001 {
		t.Fatalf("models = %+v, want weight %.3f", models, want)
	}
}

func TestPrinterServiceRefreshModelsParsesDownloadedFileWhenMoonrakerHasNoFilamentMetadata(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	client := &printerJobClientStub{
		models:   []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "elegoo.gcode", Filename: "elegoo.gcode"}},
		metadata: &domain.GcodeMetadata{Filename: "elegoo.gcode", PrintDuration: time.Hour},
		downloadData: []byte("; CURA_FILAMENT_WEIGHT_PLACEHOLDER\n" +
			"G28\n" +
			"; filament used [mm] = 19323.08\n" +
			"; filament used [g] = 58.10\n" +
			"; estimated printing time (normal mode) = 1h 43m 7s\n"),
	}
	service := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{})
	service.SetJobClient(client)

	models, err := service.RefreshModels(t.Context(), "printer-1")
	if err != nil {
		t.Fatalf("RefreshModels: %v", err)
	}
	if len(models) != 1 || models[0].EstimatedWeightG != 58.10 || !strings.HasPrefix(client.downloaded, "elegoo.gcode>") {
		t.Fatalf("models=%+v downloaded=%q", models, client.downloaded)
	}
}

func TestPrinterServiceListModelsReadsCache(t *testing.T) {
	cache := &printerModelRepoStub{list: []domain.PrinterModelFile{{PrinterID: "printer-1", Path: "cached.gcode"}}}
	client := &printerJobClientStub{modelsErr: errors.New("remote must not be called")}
	service := newTestPrinterService(&printerRepoStub{}, &spoolRepoStub{}, &printRepoStub{})
	service.SetJobClient(client)
	service.SetModelCache(cache)

	models, err := service.ListModels(t.Context(), "printer-1")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 1 || models[0].Path != "cached.gcode" {
		t.Fatalf("models = %+v", models)
	}
}

func TestPrinterServiceSyncPrinterHistoryBackfillsWithoutSpool(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{}
	client := &printerJobClientStub{
		metadata: &domain.GcodeMetadata{Filename: "cube.gcode", FilamentWeightG: 16, PreviewImagePath: "/tmp/cube.png"},
		history: []domain.PrinterHistoryJob{{
			PrinterID:        "printer-1",
			ExternalPrintID:  "remote-1",
			GcodeFilename:    "cube.gcode",
			Status:           domain.PrinterHistoryStatusCompleted,
			EstimatedWeightG: 15,
			EndedAt:          time.Unix(1000, 0),
		}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	if created != 1 || prints.created == nil || prints.created.SpoolID != "" || prints.created.ExternalPrintID != "remote-1" || prints.created.EstimatedWeightG != 16 || prints.created.ActualWeightG != 16 || prints.created.PreviewImagePath != "/tmp/cube.png" {
		t.Fatalf("sync result created=%d job=%+v", created, prints.created)
	}
}

func TestPrinterServiceSyncPrinterHistoryUsesActiveSpoolForNewJobs(t *testing.T) {
	boundary := time.Unix(1000, 0)
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{list: []domain.PrintJob{{
		ID:              "existing",
		PrinterID:       "printer-1",
		ExternalPrintID: "existing",
		GcodeFilename:   "old.gcode",
		PrintedAt:       boundary,
	}}}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{
		PrinterID:        "printer-1",
		ExternalPrintID:  "remote-new",
		GcodeFilename:    "new.gcode",
		Status:           domain.PrinterHistoryStatusCompleted,
		EstimatedWeightG: 15,
		EndedAt:          boundary.Add(time.Minute),
	}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	if created != 1 || prints.created == nil || prints.created.SpoolID != "spool-1" || prints.created.ActualWeightG != 15 {
		t.Fatalf("sync result created=%d job=%+v", created, prints.created)
	}
}

func TestPrinterServiceSyncPrinterHistoryUsesFilamentUsedForCancelledJobs(t *testing.T) {
	boundary := time.Unix(1000, 0)
	spool := &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: spool.ID}}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{list: []domain.PrintJob{{
		ID:              "existing",
		PrinterID:       "printer-1",
		ExternalPrintID: "existing",
		GcodeFilename:   "old.gcode",
		PrintedAt:       boundary,
	}}}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{
		PrinterID:        "printer-1",
		ExternalPrintID:  "remote-cancelled",
		GcodeFilename:    "cancelled.gcode",
		Status:           domain.PrinterHistoryStatusCancelled,
		FilamentUsedMM:   1000,
		EstimatedWeightG: 20,
		EndedAt:          boundary.Add(time.Minute),
	}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	wantActual := domain.WeightFromLength(1000, spool.DiameterMM, spool.DensityGCM3)
	if created != 1 || prints.created == nil || prints.created.SpoolID != spool.ID || prints.created.Status != domain.PrintStatusCancelled || math.Abs(prints.created.ActualWeightG-wantActual) > 1e-9 || prints.created.CompletionPercent <= 0 {
		t.Fatalf("sync result created=%d job=%+v want actual %.6f", created, prints.created, wantActual)
	}
}

func TestPrinterServiceSyncPrinterHistoryUpdatesExistingZeroUsage(t *testing.T) {
	spool := &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron"}}
	spools := &spoolRepoStub{spool: spool}
	prints := &printRepoStub{list: []domain.PrintJob{{
		ID:                "local-1",
		SpoolID:           spool.ID,
		PrinterID:         "printer-1",
		ExternalPrintID:   "remote-cancelled",
		GcodeFilename:     "cancelled.gcode",
		Status:            domain.PrintStatusCancelled,
		CompletionPercent: 0,
		EstimatedWeightG:  20,
		ActualWeightG:     0,
		EstimatedLengthMM: 0,
		PrintedAt:         time.Unix(1000, 0),
	}}}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{
		PrinterID:        "printer-1",
		ExternalPrintID:  "remote-cancelled",
		GcodeFilename:    "cancelled.gcode",
		Status:           domain.PrinterHistoryStatusCancelled,
		FilamentUsedMM:   1000,
		EstimatedWeightG: 20,
		EndedAt:          time.Unix(1000, 0),
	}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	wantActual := domain.WeightFromLength(1000, spool.DiameterMM, spool.DensityGCM3)
	if created != 0 || prints.usageJobID != "local-1" || prints.estimatedLength != 1000 || math.Abs(prints.actualWeight-wantActual) > 1e-9 || prints.completion <= 0 {
		t.Fatalf("created=%d usage job=%q length=%.1f actual=%.6f completion=%d", created, prints.usageJobID, prints.estimatedLength, prints.actualWeight, prints.completion)
	}
}

func TestPrinterServiceSyncPrinterHistoryImportsHistoryPages(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron"}}
	prints := &printRepoStub{}
	client := &printerJobClientStub{
		historyCount: 3,
		historyPages: map[int][]domain.PrinterHistoryJob{
			0: {
				{PrinterID: "printer-1", ExternalPrintID: "remote-1", GcodeFilename: "one.gcode", Status: domain.PrinterHistoryStatusCompleted, EstimatedWeightG: 10, EndedAt: time.Unix(3000, 0)},
				{PrinterID: "printer-1", ExternalPrintID: "remote-2", GcodeFilename: "two.gcode", Status: domain.PrinterHistoryStatusCompleted, EstimatedWeightG: 20, EndedAt: time.Unix(2000, 0)},
			},
			2: {
				{PrinterID: "printer-1", ExternalPrintID: "remote-3", GcodeFilename: "three.gcode", Status: domain.PrinterHistoryStatusCompleted, EstimatedWeightG: 30, EndedAt: time.Unix(1000, 0)},
			},
		},
	}
	service := newTestPrinterService(printers, &spoolRepoStub{}, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 0)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	if created != 3 || len(prints.createdJobs) != 3 {
		t.Fatalf("created=%d jobs=%+v", created, prints.createdJobs)
	}
	if len(client.historyStarts) != 2 || client.historyStarts[0] != 0 || client.historyStarts[1] != 2 {
		t.Fatalf("history starts = %+v", client.historyStarts)
	}
}

func TestPrinterServiceSyncPrinterHistorySkipsLiveMonitorDuplicate(t *testing.T) {
	endedAt := time.Unix(1020, 0)
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", Name: "Voron", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", DiameterMM: 1.75, DensityGCM3: 1.24}}
	prints := &printRepoStub{list: []domain.PrintJob{{
		ID:              "live-1",
		PrinterID:       "printer-1",
		ExternalPrintID: "live-generated-id",
		GcodeFilename:   "cube.gcode",
		PrintedAt:       endedAt,
	}}}
	client := &printerJobClientStub{history: []domain.PrinterHistoryJob{{
		PrinterID:        "printer-1",
		ExternalPrintID:  "moonraker-history-id",
		GcodeFilename:    "cube.gcode",
		Status:           domain.PrinterHistoryStatusCompleted,
		EstimatedWeightG: 15,
		EndedAt:          endedAt.Add(20 * time.Second),
	}}}
	service := newTestPrinterService(printers, spools, prints)
	service.SetJobClient(client)

	created, err := service.SyncPrinterHistory(t.Context(), "printer-1", 50)
	if err != nil {
		t.Fatalf("SyncPrinterHistory: %v", err)
	}
	if created != 0 || prints.created != nil {
		t.Fatalf("expected duplicate to be skipped, created=%d job=%+v", created, prints.created)
	}
}

func TestPrinterServicePrintModelTransfersBetweenPrinters(t *testing.T) {
	printers := &printerRepoStub{list: []domain.Printer{
		{ID: "source", Name: "Source", Kind: domain.PrinterKindMoonraker, Enabled: true},
		{ID: "target", Name: "Target", Kind: domain.PrinterKindMoonraker, Enabled: true, ActiveSpoolID: "spool-1"},
	}}
	printers.printer = &printers.list[0]
	getCalls := 0
	printers.getByIDFunc = func(id string) (*domain.Printer, error) {
		getCalls++
		for i := range printers.list {
			if printers.list[i].ID == id {
				return &printers.list[i], nil
			}
		}
		return nil, errors.New("missing")
	}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", TotalWeightG: 100, DiameterMM: 1.75, DensityGCM3: 1.24}, used: 10}
	client := &printerJobClientStub{metadata: &domain.GcodeMetadata{Filename: "cube.gcode", FilamentWeightG: 20}, uploadedPath: "cube.gcode"}
	service := newTestPrinterService(printers, spools, &printRepoStub{})
	service.SetJobClient(client)

	result, err := service.PrintModel(t.Context(), domain.PrintLaunchRequest{SourcePrinterID: "source", PrinterID: "target", GcodeFilename: "cube.gcode"})
	if err != nil {
		t.Fatalf("PrintModel: %v", err)
	}
	if !result.Uploaded || !result.Started || client.downloaded == "" || client.uploadedLocal == "" || getCalls < 2 {
		t.Fatalf("print model result=%+v client=%+v getCalls=%d", result, client, getCalls)
	}
}

func TestPrinterServiceStartPrinterFileChecksFilament(t *testing.T) {
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1", ActiveSpoolID: "spool-1"}}
	spools := &spoolRepoStub{spool: &domain.Spool{ID: "spool-1", TotalWeightG: 100, DiameterMM: 1.75, DensityGCM3: 1.24}, used: 60}
	client := &printerJobClientStub{metadata: &domain.GcodeMetadata{Filename: "big.gcode", FilamentWeightG: 50}}
	settings := &settingsRepoStub{setting: "50"}
	service := NewPrinterService(printers, spools, &printRepoStub{}, settings)
	service.SetJobClient(client)

	result, err := service.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "printer-1", GcodeFilename: "big.gcode"})
	if err != nil {
		t.Fatalf("StartPrinterFile: %v", err)
	}
	if !result.Filament.Blocked || client.startedFile != "" {
		t.Fatalf("expected blocked launch, result=%+v started=%q", result, client.startedFile)
	}

	spools.used = 40
	client.metadata = &domain.GcodeMetadata{Filename: "warn.gcode", FilamentWeightG: 20}
	result, err = service.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "printer-1", GcodeFilename: "warn.gcode"})
	if err != nil {
		t.Fatalf("StartPrinterFile warning: %v", err)
	}
	if !result.RequiresUserConfirmation || client.startedFile != "" {
		t.Fatalf("expected confirmation before low-filament launch, result=%+v started=%q", result, client.startedFile)
	}

	settings.setting = "30"
	result, err = service.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "printer-1", GcodeFilename: "warn.gcode"})
	if err != nil {
		t.Fatalf("StartPrinterFile with lower threshold: %v", err)
	}
	if result.RequiresUserConfirmation || !result.Started || client.startedFile != "warn.gcode" {
		t.Fatalf("expected launch without warning at lower threshold, result=%+v started=%q", result, client.startedFile)
	}

	settings.setting = "50"
	client.startedFile = ""
	result, err = service.StartPrinterFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "printer-1", GcodeFilename: "warn.gcode", AllowLowFilament: true})
	if err != nil {
		t.Fatalf("StartPrinterFile allowed warning: %v", err)
	}
	if !result.Started || client.startedFile != "warn.gcode" {
		t.Fatalf("expected started launch, result=%+v started=%q", result, client.startedFile)
	}
}

func TestPrinterServiceUploadLocalFileWithoutStarting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upload-only.gcode")
	if err := os.WriteFile(path, []byte("; input_filename = upload-only.stl\n"), 0o644); err != nil {
		t.Fatalf("writing gcode: %v", err)
	}
	printers := &printerRepoStub{printer: &domain.Printer{ID: "printer-1"}}
	client := &printerJobClientStub{uploadedPath: "upload-only.gcode"}
	service := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{})
	service.SetJobClient(client)

	result, err := service.UploadLocalFile(t.Context(), domain.PrintLaunchRequest{PrinterID: "printer-1", LocalPath: path}, false)
	if err != nil {
		t.Fatalf("UploadLocalFile: %v", err)
	}
	if !result.Uploaded || result.Started || client.uploadStarted || client.uploadedLocal != path {
		t.Fatalf("upload result=%+v client=%+v", result, client)
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
	settings.customBindings = []domain.CustomMaterial{{Name: "Wood", DryingProfileMaterial: domain.MaterialPLA}}
	if got := service.GetRecommendation(t.Context(), "Wood"); got == nil || got.Material != domain.MaterialPLA || got.TemperatureC != 99 {
		t.Fatalf("bound custom material recommendation = %+v", got)
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

func TestDryingServiceGetStatus(t *testing.T) {
	profile := domain.DryingProfile{Material: domain.MaterialPLA, TemperatureC: 45, DurationHours: 4, MaxDaysNoDry: 7}
	dryings := &dryingRepoStub{}
	service := NewDryingService(dryings, &settingsRepoStub{profiles: []domain.DryingProfile{profile}})
	spool := &domain.Spool{ID: "spool", Material: domain.MaterialPLA}

	status, err := service.GetStatus(t.Context(), spool)
	if err != nil || status.Profile == nil || !status.NeedsDrying || status.LastSession != nil {
		t.Fatalf("never-dried status = (%+v, %v)", status, err)
	}

	dryings.last = &domain.DryingSession{SpoolID: spool.ID, DriedAt: time.Now().Add(-2 * 24 * time.Hour)}
	status, err = service.GetStatus(t.Context(), spool)
	if err != nil || status.NeedsDrying || status.DaysUntilNext != 5 || status.NextDryingAt.IsZero() {
		t.Fatalf("recently-dried status = (%+v, %v)", status, err)
	}

	dryings.last = &domain.DryingSession{SpoolID: spool.ID, DriedAt: time.Now().Add(-9 * 24 * time.Hour)}
	status, err = service.GetStatus(t.Context(), spool)
	if err != nil || !status.NeedsDrying || status.DaysUntilNext >= 0 {
		t.Fatalf("overdue status = (%+v, %v)", status, err)
	}

	wantErr := errors.New("last drying failed")
	dryings.lastErr = wantErr
	if _, err := service.GetStatus(t.Context(), spool); !errors.Is(err, wantErr) {
		t.Fatalf("GetStatus error = %v, want %v", err, wantErr)
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

func TestSettingsServiceAllowLowFilamentWarning(t *testing.T) {
	repo := &settingsRepoStub{setting: "true"}
	service := NewSettingsService(repo)

	if !service.AllowLowFilamentWarning(t.Context()) {
		t.Fatal("expected low filament warning override to be enabled")
	}
	if err := service.SetAllowLowFilamentWarning(t.Context(), false); err != nil {
		t.Fatalf("SetAllowLowFilamentWarning: %v", err)
	}
	if repo.setKey != allowLowFilamentSettingKey || repo.setValue != "false" {
		t.Fatalf("setting = %q %q", repo.setKey, repo.setValue)
	}

	repo.setting = "not-bool"
	if service.AllowLowFilamentWarning(t.Context()) {
		t.Fatal("invalid bool setting should fall back to false")
	}
}

func TestSettingsServiceLowFilamentLevel(t *testing.T) {
	repo := &settingsRepoStub{setting: "50"}
	settingsService := NewSettingsService(repo)

	if got := settingsService.GetLowFilamentLevel(t.Context()); got != 50 {
		t.Fatalf("GetLowFilamentLevel() = %d, want 50", got)
	}

	for _, invalid := range []string{"invalid", "-1"} {
		repo.setting = invalid
		if got := settingsService.GetLowFilamentLevel(t.Context()); got != 0 {
			t.Errorf("GetLowFilamentLevel() for %q = %d, want 0", invalid, got)
		}
	}
	repo.getSettingErr = errors.New("read failed")
	if got := settingsService.GetLowFilamentLevel(t.Context()); got != 0 {
		t.Errorf("GetLowFilamentLevel() on repository error = %d, want 0", got)
	}
	repo.getSettingErr = nil

	if err := settingsService.SetLowFilamentLevel(t.Context(), 75); err != nil {
		t.Fatalf("SetLowFilamentLevel(): %v", err)
	}
	if repo.setKey != lowFilamentLevelSettingKey || repo.setValue != "75" {
		t.Fatalf("saved setting = (%q, %q), want (%q, %q)",
			repo.setKey, repo.setValue, lowFilamentLevelSettingKey, "75")
	}

	repo.setKey, repo.setValue = "", ""
	if err := settingsService.SetLowFilamentLevel(t.Context(), -1); err == nil {
		t.Fatal("SetLowFilamentLevel(-1) returned nil error")
	}
	if repo.setKey != "" || repo.setValue != "" {
		t.Fatalf("negative level was persisted as (%q, %q)", repo.setKey, repo.setValue)
	}

	repo.setSettingErr = errors.New("write failed")
	if err := settingsService.SetLowFilamentLevel(t.Context(), 100); !errors.Is(err, repo.setSettingErr) {
		t.Fatalf("SetLowFilamentLevel() error = %v, want %v", err, repo.setSettingErr)
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
	if err != nil || len(materials) != len(domain.BuiltInMaterials)+1 || materials[len(materials)-1] != "Wood" {
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
	materialABS := domain.MaterialABS
	materialWood := domain.Material("Wood")
	if !materialABS.IsBuiltIn() || materialWood.IsBuiltIn() {
		t.Fatal("IsBuiltIn returned unexpected result")
	}
	if !strings.Contains(service.DeleteCustomMaterial(t.Context(), "PLA").Error(), "built-in") {
		t.Fatal("built-in deletion error should be descriptive")
	}

	repo.customErr = nil
	repo.customBindings = []domain.CustomMaterial{{Name: "Wood", DryingProfileMaterial: domain.MaterialPLA}}
	repo.profiles = []domain.DryingProfile{{Material: domain.MaterialPLA, TemperatureC: 45, DurationHours: 4, MaxDaysNoDry: 30}}
	repo.upsertErr = nil
	if bindings, err := service.ListCustomMaterialBindings(t.Context()); err != nil || len(bindings) != 1 {
		t.Fatalf("ListCustomMaterialBindings() = (%+v, %v)", bindings, err)
	}
	if err := service.BindCustomMaterialDryingProfile(t.Context(), "Wood", domain.MaterialPLA); err != nil || repo.boundMaterial != "Wood" || repo.boundProfile != "PLA" {
		t.Fatalf("BindCustomMaterialDryingProfile() = %v, binding=%q->%q", err, repo.boundMaterial, repo.boundProfile)
	}
	if err := service.BindCustomMaterialDryingProfile(t.Context(), "Wood", "Missing"); err == nil {
		t.Fatal("expected missing drying profile error")
	}
	if err := service.BindCustomMaterialDryingProfile(t.Context(), domain.MaterialPLA, domain.MaterialPETG); err == nil {
		t.Fatal("expected built-in material binding error")
	}
	if err := service.CreateCustomMaterialDryingProfile(t.Context(), "Wood", &domain.DryingProfile{TemperatureC: 48, DurationHours: 7, MaxDaysNoDry: 4}); err != nil {
		t.Fatalf("CreateCustomMaterialDryingProfile: %v", err)
	}
	if repo.ownedProfile == nil || repo.ownedProfile.Material != "Wood" || repo.ownedProfile.TemperatureC != 48 {
		t.Fatalf("owned profile = %+v", repo.ownedProfile)
	}
}
