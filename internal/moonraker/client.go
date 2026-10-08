package moonraker

import (
	"context"

	"awesomeProject/internal/domain"

	"github.com/gorilla/websocket"
	api "github.com/yanefed/moonraker"
)

type Conn = api.Conn

type Dialer interface {
	Dial(ctx context.Context, printer domain.Printer) (Conn, error)
}

type WebSocketDialer struct{ Dialer *websocket.Dialer }

func (d WebSocketDialer) Dial(ctx context.Context, printer domain.Printer) (Conn, error) {
	return (api.WebSocketDialer{Dialer: d.Dialer}).Dial(ctx, printerConfig(printer))
}

func WebSocketURL(raw string) (string, error) { return api.WebSocketURL(raw) }

func printerConfig(printer domain.Printer) api.Config {
	return api.Config{URL: printer.MoonrakerURL, APIKey: printer.APIKey}
}
