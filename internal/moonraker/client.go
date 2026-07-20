package moonraker

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"awesomeProject/internal/domain"

	"github.com/gorilla/websocket"
)

type Conn interface {
	WriteJSON(v any) error
	ReadJSON(v any) error
	Close() error
}

type Dialer interface {
	Dial(ctx context.Context, printer domain.Printer) (Conn, error)
}

type WebSocketDialer struct {
	Dialer *websocket.Dialer
}

func (d WebSocketDialer) Dial(ctx context.Context, printer domain.Printer) (Conn, error) {
	wsURL, err := WebSocketURL(printer.MoonrakerURL)
	if err != nil {
		return nil, err
	}

	headers := http.Header{}
	if printer.APIKey != "" {
		headers.Set("X-Api-Key", printer.APIKey)
	}

	dialer := d.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("connecting to moonraker websocket: %w", err)
	}
	return conn, nil
}

func WebSocketURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("valid moonraker URL is required")
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("moonraker URL must use http, https, ws, or wss")
	}

	parsed.Path = "/websocket"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
