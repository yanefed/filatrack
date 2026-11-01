package main

import (
	"context"

	"github.com/yanefed/filatrack/internal/database"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/moonraker"
)

// Keep the same client for the entire session, including after reconnection.
// Remote mutations must be blocked while the database is read-only as well.
type databaseAwareJobClient struct {
	moonraker.HTTPControlClient
	database *database.Manager
}

func (c databaseAwareJobClient) StartFile(ctx context.Context, printer domain.Printer, filename string) error {
	if c.database.ReadOnly() {
		return database.ErrReadOnly
	}
	return c.HTTPControlClient.StartFile(ctx, printer, filename)
}

func (c databaseAwareJobClient) Upload(ctx context.Context, printer domain.Printer, localPath, remotePath string, startPrint bool) (domain.PrinterUploadResult, error) {
	if c.database.ReadOnly() {
		return domain.PrinterUploadResult{}, database.ErrReadOnly
	}
	return c.HTTPControlClient.Upload(ctx, printer, localPath, remotePath, startPrint)
}
