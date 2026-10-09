package moonraker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yanefed/filatrack/internal/domain"
)

type fakeConn struct {
	writes []any
	reads  []json.RawMessage
}

func (c *fakeConn) WriteJSON(v any) error {
	c.writes = append(c.writes, v)
	return nil
}

func (c *fakeConn) ReadJSON(v any) error {
	if len(c.reads) == 0 {
		return errors.New("done")
	}
	raw := c.reads[0]
	c.reads = c.reads[1:]
	target := v.(*json.RawMessage)
	*target = raw
	return nil
}

func (c *fakeConn) Close() error { return nil }

type fakeDialer struct {
	conn *fakeConn
}

func (d fakeDialer) Dial(context.Context, domain.Printer) (Conn, error) {
	return d.conn, nil
}

type fakeSink struct {
	statuses []domain.Printer
	events   []domain.PrinterPrintEvent
}

func (s *fakeSink) UpdateStatus(_ context.Context, printer *domain.Printer) error {
	s.statuses = append(s.statuses, *printer)
	return nil
}

func (s *fakeSink) RecordPrinterPrint(_ context.Context, event domain.PrinterPrintEvent) (*domain.PrintJob, error) {
	s.events = append(s.events, event)
	return &domain.PrintJob{}, nil
}

func TestMonitorSubscribesAndProcessesMessages(t *testing.T) {
	conn := &fakeConn{reads: []json.RawMessage{
		json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{}}`),
		json.RawMessage(`{"jsonrpc":"2.0","method":"notify_status_update","params":[{"print_stats":{"state":"printing","filename":"part.gcode"},"virtual_sdcard":{"progress":0.5}}]}`),
		json.RawMessage(`{"jsonrpc":"2.0","method":"notify_status_update","params":[{"print_stats":{"state":"complete","filename":"part.gcode","filament_used":1000,"print_duration":60},"virtual_sdcard":{"progress":1}}]}`),
		json.RawMessage(`{"jsonrpc":"2.0","method":"notify_history_changed","params":[{"action":"finished","job":{"job_id":"000042","filename":"part.gcode","status":"completed","end_time":1060,"filament_used":1000,"print_duration":60}}]}`),
	}}
	sink := &fakeSink{}
	monitor := Monitor{
		Printer:  domain.Printer{ID: "printer-1"},
		Dialer:   fakeDialer{conn: conn},
		Sink:     sink,
		Metadata: fakeMetadataFetcher{metadata: FileMetadata{FilamentWeightTotal: 9.5}},
		Now:      func() time.Time { return time.Unix(1000, 0) },
	}

	err := monitor.Run(context.Background())
	if err == nil || err.Error() != "reading moonraker websocket: done" {
		t.Fatalf("Run error = %v", err)
	}
	if len(conn.writes) != 1 {
		t.Fatalf("writes = %d, want subscribe request", len(conn.writes))
	}
	if len(sink.statuses) != 2 || sink.statuses[0].State != domain.PrinterStatePrinting {
		t.Fatalf("statuses = %+v", sink.statuses)
	}
	if len(sink.events) != 1 || sink.events[0].ExternalPrintID != "000042" || sink.events[0].PrintedAt != time.Unix(1060, 0) || sink.events[0].Status != domain.PrintStatusSuccess || sink.events[0].ActualLengthMM != 1000 || sink.events[0].EstimatedWeightG != 9.5 {
		t.Fatalf("events = %+v", sink.events)
	}
}

func TestMonitorDoesNotPersistClientLocalIdentity(t *testing.T) {
	conn := &fakeConn{reads: []json.RawMessage{
		json.RawMessage(`{"method":"notify_status_update","params":[{"print_stats":{"state":"printing","filename":"part.gcode"}}]}`),
		json.RawMessage(`{"method":"notify_status_update","params":[{"print_stats":{"state":"complete","filename":"part.gcode"}}]}`),
	}}
	sink := &fakeSink{}
	err := (Monitor{Printer: domain.Printer{ID: "p"}, Dialer: fakeDialer{conn: conn}, Sink: sink}).Run(t.Context())
	if err == nil || len(sink.events) != 0 {
		t.Fatalf("events=%+v error=%v", sink.events, err)
	}
}

func TestMonitorAcceptsFinishedHistoryBeforeStatus(t *testing.T) {
	conn := &fakeConn{reads: []json.RawMessage{
		json.RawMessage(`{"method":"notify_status_update","params":[{"print_stats":{"state":"printing","filename":"part.gcode"},"virtual_sdcard":{"progress":0.42}}]}`),
		json.RawMessage(`{"method":"notify_history_changed","params":[{"action":"finished","job":{"job_id":"000043","filename":"part.gcode","status":"cancelled","end_time":1060,"filament_used":1000}}]}`),
		json.RawMessage(`{"method":"notify_status_update","params":[{"print_stats":{"state":"cancelled","filename":"part.gcode"}}]}`),
	}}
	sink := &fakeSink{}
	err := (Monitor{Printer: domain.Printer{ID: "p"}, Dialer: fakeDialer{conn: conn}, Sink: sink}).Run(t.Context())
	if err == nil || len(sink.events) != 1 || sink.events[0].CompletionPercent != 42 || sink.events[0].ExternalPrintID != "000043" {
		t.Fatalf("events=%+v error=%v", sink.events, err)
	}
}
