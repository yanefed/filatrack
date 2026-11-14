package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

type modelMemoryRepo struct {
	saves int
	mu    sync.Mutex
	data  map[string][]domain.PrinterModelFile
}

func (r *modelMemoryRepo) List(_ context.Context, id *string) ([]domain.PrinterModelFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != nil {
		return slices.Clone(r.data[*id]), nil
	}
	var all []domain.PrinterModelFile
	for _, files := range r.data {
		all = append(all, files...)
	}
	return all, nil
}
func (r *modelMemoryRepo) SaveForPrinter(_ context.Context, id string, files []domain.PrinterModelFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saves++
	r.data[id] = slices.Clone(files)
	return nil
}

type catalogClient struct {
	printerJobClientStub
	mu                         sync.Mutex
	files                      map[string][]domain.PrinterModelFile
	metadata                   domain.GcodeMetadata
	listErr                    error
	metadataErr                error
	downloadErr                error
	listed, fetched, downloads int
	onList                     func(context.Context, domain.Printer) error
	onMetadata                 func(context.Context, domain.Printer, string) error
}

func (c *catalogClient) ListModels(ctx context.Context, p domain.Printer) ([]domain.PrinterModelFile, error) {
	c.mu.Lock()
	c.listed++
	files, err := slices.Clone(c.files[p.ID]), c.listErr
	c.mu.Unlock()
	if c.onList != nil {
		if err := c.onList(ctx, p); err != nil {
			return nil, err
		}
	}
	return files, err
}
func (c *catalogClient) FetchMetadata(ctx context.Context, p domain.Printer, path string) (*domain.GcodeMetadata, error) {
	c.mu.Lock()
	c.fetched++
	metadata, err := c.metadata, c.metadataErr
	c.mu.Unlock()
	if c.onMetadata != nil {
		if err := c.onMetadata(ctx, p, path); err != nil {
			return nil, err
		}
	}
	return &metadata, err
}
func (c *catalogClient) DownloadFile(_ context.Context, _ domain.Printer, _, path string) error {
	c.mu.Lock()
	c.downloads++
	err := c.downloadErr
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte("G28\n"), 0600)
}
func (c *catalogClient) counts() (int, int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listed, c.fetched, c.downloads
}

func catalogFixture(t *testing.T) (*PrinterService, *catalogClient, *modelMemoryRepo, *printerRepoStub) {
	t.Helper()
	printer := domain.Printer{ID: "p", Name: "Printer", Enabled: true, Kind: domain.PrinterKindMoonraker, MoonrakerURL: "http://printer"}
	printers := &printerRepoStub{printer: &printer, list: []domain.Printer{printer}}
	repo := &modelMemoryRepo{data: make(map[string][]domain.PrinterModelFile)}
	client := &catalogClient{files: map[string][]domain.PrinterModelFile{"p": {{Path: "part.gcode", ModifiedAt: time.Unix(100, 0), SizeBytes: 20}}}, metadata: domain.GcodeMetadata{FilamentWeightG: 10}}
	svc := newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{}, PrinterOptions{JobClient: client, Models: repo})
	return svc, client, repo, printers
}

func TestCatalogRequestsOnlyChangedFilesAndSurvivesServiceRestart(t *testing.T) {
	svc, client, repo, printers := catalogFixture(t)
	for i := 1; i < 100; i++ {
		client.files["p"] = append(client.files["p"], domain.PrinterModelFile{Path: fmt.Sprintf("%d.gcode", i), ModifiedAt: time.Unix(100, 0), SizeBytes: 20})
	}
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	svc = newTestPrinterService(printers, &spoolRepoStub{}, &printRepoStub{}, PrinterOptions{JobClient: client, Models: repo})
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	if lists, fetch, download := client.counts(); lists != 2 || fetch != 100 || download != 0 {
		t.Fatalf("counts = %d/%d/%d", lists, fetch, download)
	}
	if repo.saves != 1 {
		t.Fatalf("unchanged catalog was rewritten: %d saves", repo.saves)
	}
	client.files["p"][0].SizeBytes++
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	if _, fetch, _ := client.counts(); fetch != 101 {
		t.Fatalf("fetches=%d", fetch)
	}
	result, err := svc.RefreshModelCatalog(t.Context(), ModelRefreshFull)
	if err != nil || len(result.Failures) > 0 {
		t.Fatalf("force: %+v %v", result.Failures, err)
	}
	if _, fetch, _ := client.counts(); fetch != 201 {
		t.Fatalf("force fetches=%d", fetch)
	}
	printers.printer.MoonrakerURL = "http://replacement"
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	if _, fetch, _ := client.counts(); fetch != 301 {
		t.Fatalf("changed origin fetches=%d", fetch)
	}
}

func TestCatalogChecksUnknownFieldsOnceAndRetriesFailures(t *testing.T) {
	svc, client, _, _ := catalogFixture(t)
	client.metadata = domain.GcodeMetadata{}
	client.downloadErr = errors.New("download failed")
	if _, err := svc.RefreshModels(t.Context(), "p"); err == nil {
		t.Fatal("expected download failure")
	}
	client.downloadErr = nil
	for range 3 {
		if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
			t.Fatal(err)
		}
	}
	if _, fetch, downloads := client.counts(); fetch != 2 || downloads != 2 {
		t.Fatalf("fetch=%d downloads=%d", fetch, downloads)
	}
	if _, err := svc.RefreshModelCatalog(t.Context(), ModelRefreshFull); err != nil {
		t.Fatal(err)
	}
	if _, fetch, downloads := client.counts(); fetch != 3 || downloads != 3 {
		t.Fatalf("force fetch=%d downloads=%d", fetch, downloads)
	}
}

func TestCatalogReplacementCannotInheritOldMetadata(t *testing.T) {
	svc, client, _, _ := catalogFixture(t)
	client.metadata.PreviewImageHash = "old-image"
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	client.files["p"][0].ModifiedAt = time.Unix(200, 0)
	client.metadataErr = errors.New("metadata unavailable")
	models, err := svc.RefreshModels(t.Context(), "p")
	if err == nil || len(models) != 1 {
		t.Fatalf("%+v %v", models, err)
	}
	if models[0].EstimatedWeightG != 0 || models[0].PreviewImageHash != "" || models[0].MetadataReady {
		t.Fatalf("stale data: %+v", models[0])
	}
	client.metadataErr = nil
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	if _, fetch, _ := client.counts(); fetch != 3 {
		t.Fatalf("retry fetch=%d", fetch)
	}
}

func TestCatalogFailedListPreservesCacheAndEmptyListRemovesFiles(t *testing.T) {
	svc, client, repo, _ := catalogFixture(t)
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	client.listErr = errors.New("offline")
	result, err := svc.RefreshModelCatalog(t.Context(), ModelRefreshChanges)
	if err != nil || len(result.Models) != 1 || len(result.Failures) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	client.listErr = nil
	client.files["p"] = nil
	if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
		t.Fatal(err)
	}
	id := "p"
	files, _ := repo.List(t.Context(), &id)
	if len(files) != 0 {
		t.Fatalf("deleted files still present: %+v", files)
	}
}

func TestCatalogRecalculatesWeightWithoutRequests(t *testing.T) {
	svc, client, _, printers := catalogFixture(t)
	spools := &spoolRepoStub{spool: &domain.Spool{DiameterMM: 1.75, DensityGCM3: 1.24}}
	svc.printerModels.spools = spools
	printers.printer.ActiveSpoolID = "spool"
	client.metadata = domain.GcodeMetadata{FilamentLengthMM: 1000}
	first, err := svc.RefreshModels(t.Context(), "p")
	if err != nil {
		t.Fatal(err)
	}
	spools.spool.DensityGCM3 *= 2
	second, err := svc.ListModels(t.Context(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if second[0].EstimatedWeightG != 2*first[0].EstimatedWeightG {
		t.Fatalf("weight did not change: %v / %v", first, second)
	}
	if lists, fetch, _ := client.counts(); lists != 1 || fetch != 1 {
		t.Fatalf("unexpected requests: %d/%d", lists, fetch)
	}
	client.metadata.FilamentWeightG = 17
	if _, err := svc.RefreshModelCatalog(t.Context(), ModelRefreshFull); err != nil {
		t.Fatal(err)
	}
	models, err := svc.ListModels(t.Context(), "p")
	if err != nil || models[0].EstimatedWeightG != 17 {
		t.Fatalf("slicer weight: %+v %v", models, err)
	}
}

func TestCatalogGlobalConcurrencyLimits(t *testing.T) {
	svc, client, _, printers := catalogFixture(t)
	printers.list = nil
	for i := range 4 {
		p := *printers.printer
		p.ID = fmt.Sprint(i)
		printers.list = append(printers.list, p)
		client.files[p.ID] = []domain.PrinterModelFile{{Path: "a"}, {Path: "b"}, {Path: "c"}}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	listGate, metadataGate := make(chan struct{}), make(chan struct{})
	listed, metadata := make(chan struct{}, 4), make(chan struct{}, 12)
	var listActive, listMax, metadataActive, metadataMax atomic.Int32
	client.onList = func(ctx context.Context, _ domain.Printer) error {
		n := listActive.Add(1)
		updateAtomicMaximum(&listMax, n)
		defer listActive.Add(-1)
		listed <- struct{}{}
		select {
		case <-listGate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	client.onMetadata = func(ctx context.Context, _ domain.Printer, _ string) error {
		n := metadataActive.Add(1)
		updateAtomicMaximum(&metadataMax, n)
		defer metadataActive.Add(-1)
		metadata <- struct{}{}
		select {
		case <-metadataGate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() {
		result, err := svc.RefreshModelCatalog(ctx, ModelRefreshChanges)
		if err == nil {
			err = refreshFailuresError(result.Failures)
		}
		done <- err
	}()
	for range 2 {
		select {
		case <-listed:
		case <-ctx.Done():
			t.Fatal("printers did not run concurrently")
		}
	}
	close(listGate)
	for range 4 {
		select {
		case <-metadata:
		case <-ctx.Done():
			t.Fatal("metadata did not run concurrently")
		}
	}
	close(metadataGate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if listMax.Load() != 2 || metadataMax.Load() != 4 {
		t.Fatalf("concurrency: printers=%d files=%d", listMax.Load(), metadataMax.Load())
	}
}

func TestCatalogSerializesUploadCacheAndSupportsCancellation(t *testing.T) {
	svc, client, repo, _ := catalogFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	client.onList = func(ctx context.Context, _ domain.Printer) error {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() { _, err := svc.RefreshModels(t.Context(), "p"); done <- err }()
	<-entered
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := svc.cacheUploadedModel(cancelled, "p", "upload.gcode", "", &domain.GcodeMetadata{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting writer: %v", err)
	}
	path := filepath.Join(t.TempDir(), "upload.gcode")
	if err := os.WriteFile(path, []byte("G28\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uploaded := make(chan error, 1)
	go func() {
		uploaded <- svc.cacheUploadedModel(t.Context(), "p", "upload.gcode", path, &domain.GcodeMetadata{FilamentWeightG: 15})
	}()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-uploaded; err != nil {
		t.Fatal(err)
	}
	id := "p"
	files, _ := repo.List(t.Context(), &id)
	if len(files) != 2 {
		t.Fatalf("lost update: %+v", files)
	}
}

func TestUploadedGcodeIsNotDownloadedAgain(t *testing.T) {
	svc, client, _, _ := catalogFixture(t)
	path := filepath.Join(t.TempDir(), "part.gcode")
	if err := os.WriteFile(path, []byte("G28\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client.files["p"][0].SizeBytes = 4
	client.metadata = domain.GcodeMetadata{}
	if err := svc.cacheUploadedModel(t.Context(), "p", "part.gcode", path, &domain.GcodeMetadata{FilamentWeightG: 12}); err != nil {
		t.Fatal(err)
	}
	models, err := svc.RefreshModels(t.Context(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].EstimatedWeightG != 12 || !models[0].MetadataReady {
		t.Fatalf("uploaded metadata: %+v", models)
	}
	if _, _, downloads := client.counts(); downloads != 0 {
		t.Fatal("downloaded G-code already parsed during upload")
	}
}

func TestCatalogPartialPrinterFailureDoesNotHideOtherResults(t *testing.T) {
	svc, client, _, printers := catalogFixture(t)
	other := *printers.printer
	other.ID = "other"
	other.Name = "Other printer"
	printers.list = append(printers.list, other)
	client.files[other.ID] = []domain.PrinterModelFile{{Path: "other.gcode", ModifiedAt: time.Unix(100, 0)}}
	failed := errors.New("printer unavailable")
	client.onList = func(_ context.Context, p domain.Printer) error {
		if p.ID == "p" {
			return failed
		}
		return nil
	}
	result, err := svc.RefreshModelCatalog(t.Context(), ModelRefreshChanges)
	if err != nil || len(result.Models) != 1 || result.Models[0].PrinterID != "other" || len(result.Failures) != 1 || !errors.Is(result.Failures[0], failed) {
		t.Fatalf("partial result: %+v %v", result, err)
	}
}

func TestCatalogWithoutModificationTimeRechecksMetadata(t *testing.T) {
	svc, client, _, _ := catalogFixture(t)
	client.files["p"][0].ModifiedAt = time.Time{}
	for range 2 {
		if _, err := svc.RefreshModels(t.Context(), "p"); err != nil {
			t.Fatal(err)
		}
	}
	if _, fetch, _ := client.counts(); fetch != 2 {
		t.Fatalf("unverifiable file was cached: %d", fetch)
	}
}
