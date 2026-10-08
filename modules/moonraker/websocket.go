package moonraker

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

// Conn is the JSON message transport used by a status subscription.
type Conn interface {
	WriteJSON(v any) error
	ReadJSON(v any) error
	Close() error
}

// Dialer opens a WebSocket connection to a Moonraker server.
type Dialer interface {
	Dial(ctx context.Context, server Config) (Conn, error)
}

// WebSocketDialer uses gorilla/websocket. A nil Dialer uses its default dialer.
type WebSocketDialer struct {
	Dialer *websocket.Dialer
}

// Dial authenticates and opens a connection. ctx bounds the handshake only.
// Close the returned connection to interrupt subsequent reads.
func (d WebSocketDialer) Dial(ctx context.Context, server Config) (Conn, error) {
	wsURL, err := WebSocketURL(server.URL)
	if err != nil {
		return nil, err
	}

	headers := http.Header{}
	if server.APIKey != "" {
		headers.Set("X-Api-Key", server.APIKey)
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

// WebSocketURL converts a server URL to its WebSocket endpoint.
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
