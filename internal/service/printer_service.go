package service

import (
	"log/slog"

	"github.com/yanefed/filatrack/internal/port"
)

const metadataSyncConcurrency = 4

// PrinterService exposes printer workflows while each component owns its state.
type PrinterService struct {
	*printerManagement
	*printerHistory
	*printerModels
	*printerLaunch
}

// PrinterOptions contains optional adapters. Without a client remote workflows
// return an error; without a model repository they operate without a catalog cache.
type PrinterOptions struct {
	Logger    *slog.Logger
	Checker   port.ConnectionChecker
	JobClient port.PrinterJobClient
	Models    port.PrinterModelRepository
	Previews  *PreviewImageStore
}

func NewPrinterService(
	printers port.PrinterRepository,
	spools port.SpoolRepository,
	prints port.PrintJobRepository,
	settings port.SettingsRepository,
	options PrinterOptions,
) *PrinterService {
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("service", "printer")
	catalog := &printerModels{
		logger: logger, printers: printers, spools: spools,
		models: options.Models, previews: options.Previews, jobClient: options.JobClient,
	}
	return &PrinterService{
		printerManagement: &printerManagement{printers: printers, spools: spools, checker: options.Checker},
		printerHistory: &printerHistory{
			printers: printers, spools: spools, prints: prints, settings: settings,
			previews: options.Previews, jobClient: options.JobClient,
		},
		printerModels: catalog,
		printerLaunch: &printerLaunch{
			logger: logger, printers: printers, spools: spools, settings: settings,
			jobClient: options.JobClient, catalog: catalog,
		},
	}
}
