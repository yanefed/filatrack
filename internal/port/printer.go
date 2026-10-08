// Package port defines the external dependency contracts used by application
// services. Adapters implement these contracts; their data belongs to domain.
package port

import (
	"context"
	"time"

	"awesomeProject/internal/domain"
)

// ConnectionChecker проверяет доступность принтера.
type ConnectionChecker interface {
	Check(ctx context.Context, printer domain.Printer) error
}

// PrinterJobClient предоставляет операции с файлами и историей принтера.
type PrinterJobClient interface {
	ListHistory(ctx context.Context, printer domain.Printer, limit int) ([]domain.PrinterHistoryJob, error)
	ListModels(ctx context.Context, printer domain.Printer) ([]domain.PrinterModelFile, error)
	FetchMetadata(ctx context.Context, printer domain.Printer, filename string) (*domain.GcodeMetadata, error)
	StartFile(ctx context.Context, printer domain.Printer, filename string) error
	DownloadFile(ctx context.Context, printer domain.Printer, filename, localPath string) error
	Upload(ctx context.Context, printer domain.Printer, localPath, remotePath string, startPrint bool) (domain.PrinterUploadResult, error)
}

// PrinterHistoryPageClient добавляет постраничное чтение истории.
type PrinterHistoryPageClient interface {
	ListHistoryPage(ctx context.Context, printer domain.Printer, start, limit int) ([]domain.PrinterHistoryJob, int, error)
}

// IncrementalHistoryClient добавляет чтение истории с временной границы.
type IncrementalHistoryClient interface {
	ListHistoryPageSince(context.Context, domain.Printer, int, int, time.Time) ([]domain.PrinterHistoryJob, int, error)
}
