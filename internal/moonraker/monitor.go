package moonraker

import (
	"awesomeProject/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Sink interface {
	UpdateStatus(ctx context.Context, printer *domain.Printer) error
	RecordPrinterPrint(ctx context.Context, event domain.PrinterPrintEvent) (*domain.PrintJob, error)
}

type Monitor struct {
	Printer  domain.Printer
	Dialer   Dialer
	Sink     Sink
	Metadata MetadataFetcher
	Now      func() time.Time
}

func (m Monitor) Run(ctx context.Context) error {
	if m.Dialer == nil {
		return errors.New("moonraker dialer is required")
	}
	if m.Sink == nil {
		return errors.New("moonraker sink is required")
	}
	now := m.Now
	if now == nil {
		now = time.Now
	}

	conn, err := m.Dialer.Dial(ctx, m.Printer)
	if err != nil {
		return err
	}
	defer func(conn Conn) {
		_ = conn.Close()
	}(conn)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	if err := conn.WriteJSON(SubscribeRequest()); err != nil {
		return fmt.Errorf("subscribing to moonraker objects: %w", err)
	}

	processor := NewProcessor(m.Printer.ID)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var raw json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return fmt.Errorf("reading moonraker websocket: %w", err)
		}

		update, ok, err := DecodeStatusUpdate(raw)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		printer, event := processor.Apply(update, now())
		if err := m.Sink.UpdateStatus(ctx, &printer); err != nil {
			continue
		}
		if event != nil {
			if m.Metadata != nil {
				enrichEventWithMetadata(ctx, m.Metadata, m.Printer, event)
			}
			if _, err := m.Sink.RecordPrinterPrint(ctx, *event); err != nil {
				return fmt.Errorf("recording printer print: %w", err)
			}
		}
	}
}

func enrichEventWithMetadata(ctx context.Context, fetcher MetadataFetcher, printer domain.Printer, event *domain.PrinterPrintEvent) {
	metadata, err := fetcher.Fetch(ctx, printer, event.GcodeFilename)
	if err != nil {
		return
	}
	if metadata.FilamentWeightTotal > 0 {
		event.EstimatedWeightG = metadata.FilamentWeightTotal
	}
	if metadata.FilamentTotalMM > 0 {
		event.EstimatedLengthMM = metadata.FilamentTotalMM
	}
	if metadata.EstimatedTime > 0 && event.PrintDuration == 0 {
		event.PrintDuration = metadata.EstimatedTime
	}
	if metadata.NozzleTempC > 0 {
		event.NozzleTempC = metadata.NozzleTempC
	}
	if metadata.BedTempC > 0 {
		event.BedTempC = metadata.BedTempC
	}
	if metadata.Slicer != "" {
		event.Slicer = metadata.Slicer
	}
	if metadata.PreviewImagePath != "" {
		event.PreviewImagePath = metadata.PreviewImagePath
	}
	if metadata.PreviewImageHash != "" {
		event.PreviewImageHash = metadata.PreviewImageHash
	}
	if len(metadata.PreviewImageData) > 0 {
		event.PreviewImageData = metadata.PreviewImageData
	}
}
