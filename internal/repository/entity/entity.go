package entity

import (
	"database/sql"
	"time"

	"awesomeProject/internal/domain"
)

type Scanner interface {
	Scan(dest ...any) error
}

func NullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func NullTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: !value.IsZero()}
}

func BoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

type Spool struct {
	ID           string
	DisplayCode  string
	Manufacturer string
	Material     string
	Color        string
	ColorHex     string
	DiameterMM   float64
	TotalWeightG float64
	Price        float64
	DensityGCM3  float64
	PurchasedAt  sql.NullTime
	Archived     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func SpoolFromDomain(spool domain.Spool) Spool {
	return Spool{
		ID: spool.ID, DisplayCode: spool.DisplayCode, Manufacturer: spool.Manufacturer,
		Material: string(spool.Material), Color: spool.Color, ColorHex: spool.ColorHex,
		DiameterMM: spool.DiameterMM, TotalWeightG: spool.TotalWeightG, Price: spool.Price,
		DensityGCM3: spool.DensityGCM3, PurchasedAt: NullTime(spool.PurchasedAt),
		Archived: BoolInt(spool.Archived), CreatedAt: spool.CreatedAt, UpdatedAt: spool.UpdatedAt,
	}
}

func (spool Spool) Domain() domain.Spool {
	result := domain.Spool{
		ID: spool.ID, DisplayCode: spool.DisplayCode, Manufacturer: spool.Manufacturer,
		Material: domain.Material(spool.Material), Color: spool.Color, ColorHex: spool.ColorHex,
		DiameterMM: spool.DiameterMM, TotalWeightG: spool.TotalWeightG, Price: spool.Price,
		DensityGCM3: spool.DensityGCM3, Archived: spool.Archived != 0,
		CreatedAt: spool.CreatedAt, UpdatedAt: spool.UpdatedAt,
	}
	if spool.PurchasedAt.Valid {
		result.PurchasedAt = spool.PurchasedAt.Time
	}
	return result
}

func ScanSpool(scanner Scanner) (Spool, error) {
	var spool Spool
	err := scanner.Scan(
		&spool.ID, &spool.DisplayCode, &spool.Manufacturer, &spool.Material,
		&spool.Color, &spool.ColorHex, &spool.DiameterMM, &spool.TotalWeightG,
		&spool.Price, &spool.DensityGCM3, &spool.PurchasedAt, &spool.Archived,
		&spool.CreatedAt, &spool.UpdatedAt,
	)
	return spool, err
}

type PrintJob struct {
	PricePerGram  float64
	CostCurrency  string
	UsageMeasured bool

	ID                string
	SpoolID           sql.NullString
	PrinterID         sql.NullString
	ExternalPrintID   string
	GcodeFilename     string
	ModelName         string
	Status            string
	CompletionPercent int
	EstimatedWeightG  float64
	ActualWeightG     float64
	EstimatedLengthMM float64
	PrintDurationSec  int64
	NozzleTempC       int
	BedTempC          int
	Slicer            string
	PreviewImagePath  string
	Notes             string
	PrintedAt         time.Time
	CreatedAt         time.Time
}

func PrintJobFromDomain(job domain.PrintJob) PrintJob {
	return PrintJob{
		ID: job.ID, SpoolID: NullString(job.SpoolID), PrinterID: NullString(job.PrinterID),
		ExternalPrintID: job.ExternalPrintID, GcodeFilename: job.GcodeFilename,
		ModelName: job.ModelName, Status: string(job.Status), CompletionPercent: job.CompletionPercent,
		EstimatedWeightG: job.EstimatedWeightG, ActualWeightG: job.ActualWeightG,
		EstimatedLengthMM: job.EstimatedLengthMM, PrintDurationSec: int64(job.PrintDuration.Seconds()),
		NozzleTempC: job.NozzleTempC, BedTempC: job.BedTempC, Slicer: job.Slicer,
		PreviewImagePath: job.PreviewImagePath, Notes: job.Notes,
		PrintedAt: job.PrintedAt, CreatedAt: job.CreatedAt,
		UsageMeasured: job.UsageMeasured,
	}
}

func (job PrintJob) Domain() domain.PrintJob {
	return domain.PrintJob{
		ID: job.ID, SpoolID: job.SpoolID.String, PrinterID: job.PrinterID.String,
		ExternalPrintID: job.ExternalPrintID, GcodeFilename: job.GcodeFilename,
		ModelName: job.ModelName, Status: domain.PrintStatus(job.Status),
		CompletionPercent: job.CompletionPercent, EstimatedWeightG: job.EstimatedWeightG,
		ActualWeightG: job.ActualWeightG, EstimatedLengthMM: job.EstimatedLengthMM,
		PrintDuration: time.Duration(job.PrintDurationSec) * time.Second,
		NozzleTempC:   job.NozzleTempC, BedTempC: job.BedTempC, Slicer: job.Slicer,
		PreviewImagePath: job.PreviewImagePath, Notes: job.Notes,
		PrintedAt: job.PrintedAt, CreatedAt: job.CreatedAt,
		PricePerGram: job.PricePerGram, CostCurrency: job.CostCurrency,
		UsageMeasured: job.UsageMeasured,
	}
}

func ScanPrintJob(scanner Scanner) (PrintJob, error) {
	var job PrintJob
	err := scanner.Scan(
		&job.ID, &job.SpoolID, &job.PrinterID, &job.ExternalPrintID,
		&job.GcodeFilename, &job.ModelName, &job.Status, &job.CompletionPercent,
		&job.EstimatedWeightG, &job.ActualWeightG, &job.EstimatedLengthMM,
		&job.PrintDurationSec, &job.NozzleTempC, &job.BedTempC, &job.Slicer,
		&job.PreviewImagePath, &job.Notes, &job.PrintedAt, &job.CreatedAt,
		&job.PricePerGram, &job.CostCurrency, &job.UsageMeasured,
	)
	return job, err
}

type Printer struct {
	ID            string
	Name          string
	Kind          string
	MoonrakerURL  string
	APIKey        string
	Enabled       int
	ActiveSpoolID sql.NullString
	State         string
	CurrentFile   string
	Progress      float64
	LastSeenAt    sql.NullTime
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func PrinterFromDomain(printer domain.Printer) Printer {
	return Printer{
		ID: printer.ID, Name: printer.Name, Kind: string(printer.Kind),
		MoonrakerURL: printer.MoonrakerURL, APIKey: printer.APIKey, Enabled: BoolInt(printer.Enabled),
		ActiveSpoolID: NullString(printer.ActiveSpoolID), State: string(printer.State),
		CurrentFile: printer.CurrentFile, Progress: printer.Progress,
		LastSeenAt: NullTime(printer.LastSeenAt), CreatedAt: printer.CreatedAt,
		UpdatedAt: printer.UpdatedAt,
	}
}

func (printer Printer) Domain() domain.Printer {
	result := domain.Printer{
		ID: printer.ID, Name: printer.Name, Kind: domain.PrinterKind(printer.Kind),
		MoonrakerURL: printer.MoonrakerURL, APIKey: printer.APIKey,
		Enabled: printer.Enabled != 0, ActiveSpoolID: printer.ActiveSpoolID.String,
		State: domain.PrinterState(printer.State), CurrentFile: printer.CurrentFile,
		Progress: printer.Progress, CreatedAt: printer.CreatedAt, UpdatedAt: printer.UpdatedAt,
	}
	if printer.LastSeenAt.Valid {
		result.LastSeenAt = printer.LastSeenAt.Time
	}
	return result
}

func ScanPrinter(scanner Scanner) (Printer, error) {
	var printer Printer
	err := scanner.Scan(
		&printer.ID, &printer.Name, &printer.Kind, &printer.MoonrakerURL,
		&printer.APIKey, &printer.Enabled, &printer.ActiveSpoolID, &printer.State,
		&printer.CurrentFile, &printer.Progress, &printer.LastSeenAt,
		&printer.CreatedAt, &printer.UpdatedAt,
	)
	return printer, err
}

type DryingSession struct {
	ID            string
	SpoolID       string
	TemperatureC  int
	DurationHours float64
	DriedAt       time.Time
	CreatedAt     time.Time
}

func DryingSessionFromDomain(session domain.DryingSession) DryingSession {
	return DryingSession{
		ID: session.ID, SpoolID: session.SpoolID, TemperatureC: session.TemperatureC,
		DurationHours: session.DurationHours, DriedAt: session.DriedAt, CreatedAt: session.CreatedAt,
	}
}

func (session DryingSession) Domain() domain.DryingSession {
	return domain.DryingSession{
		ID: session.ID, SpoolID: session.SpoolID, TemperatureC: session.TemperatureC,
		DurationHours: session.DurationHours, DriedAt: session.DriedAt, CreatedAt: session.CreatedAt,
	}
}

func ScanDryingSession(scanner Scanner) (DryingSession, error) {
	var session DryingSession
	err := scanner.Scan(
		&session.ID, &session.SpoolID, &session.TemperatureC,
		&session.DurationHours, &session.DriedAt, &session.CreatedAt,
	)
	return session, err
}

type PrinterModel struct {
	PrinterID        string
	Path             string
	Filename         string
	ModifiedAt       sql.NullTime
	SizeBytes        int64
	EstimatedWeightG float64
	PrintDurationSec int64
	Slicer           string
	PreviewImagePath string
	CachedAt         time.Time
}

func PrinterModelFromDomain(model domain.PrinterModelFile) PrinterModel {
	return PrinterModel{
		PrinterID: model.PrinterID, Path: model.Path, Filename: model.Filename,
		ModifiedAt: NullTime(model.ModifiedAt), SizeBytes: model.SizeBytes,
		EstimatedWeightG: model.EstimatedWeightG,
		PrintDurationSec: int64(model.PrintDuration.Seconds()), Slicer: model.Slicer,
		PreviewImagePath: model.PreviewImagePath, CachedAt: model.CachedAt,
	}
}

func (model PrinterModel) Domain() domain.PrinterModelFile {
	result := domain.PrinterModelFile{
		PrinterID: model.PrinterID, Path: model.Path, Filename: model.Filename,
		SizeBytes: model.SizeBytes, EstimatedWeightG: model.EstimatedWeightG,
		PrintDuration: time.Duration(model.PrintDurationSec) * time.Second,
		Slicer:        model.Slicer, PreviewImagePath: model.PreviewImagePath, CachedAt: model.CachedAt,
	}
	if model.ModifiedAt.Valid {
		result.ModifiedAt = model.ModifiedAt.Time
	}
	return result
}

func ScanPrinterModel(scanner Scanner) (PrinterModel, error) {
	var model PrinterModel
	err := scanner.Scan(
		&model.PrinterID, &model.Path, &model.Filename, &model.ModifiedAt,
		&model.SizeBytes, &model.EstimatedWeightG, &model.PrintDurationSec,
		&model.Slicer, &model.PreviewImagePath, &model.CachedAt,
	)
	return model, err
}

type Material struct {
	Name                string
	DensityGCM3         float64
	DryingTemperatureC  int
	DryingDurationHours float64
	MaxDaysNoDry        int
}

func MaterialFromDomain(material domain.MaterialDefinition) Material {
	return Material{
		Name: string(material.Name), DensityGCM3: material.DensityGCM3,
		DryingTemperatureC:  material.DryingTemperatureC,
		DryingDurationHours: material.DryingDurationHours,
		MaxDaysNoDry:        material.MaxDaysNoDry,
	}
}

func (material Material) Domain() domain.MaterialDefinition {
	return domain.MaterialDefinition{
		Name: domain.Material(material.Name), DensityGCM3: material.DensityGCM3,
		DryingTemperatureC:  material.DryingTemperatureC,
		DryingDurationHours: material.DryingDurationHours,
		MaxDaysNoDry:        material.MaxDaysNoDry,
	}
}

func ScanMaterial(scanner Scanner) (Material, error) {
	var material Material
	err := scanner.Scan(
		&material.Name, &material.DensityGCM3, &material.DryingTemperatureC,
		&material.DryingDurationHours, &material.MaxDaysNoDry,
	)
	return material, err
}
