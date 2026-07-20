package moonraker

import "testing"

func TestWebSocketURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "http://printer.local", want: "ws://printer.local/websocket"},
		{in: "https://printer.local:7125/api", want: "wss://printer.local:7125/websocket"},
		{in: "ws://printer.local/websocket?x=1", want: "ws://printer.local/websocket"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := WebSocketURL(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("WebSocketURL() = (%q, %v), want %q", got, err, tt.want)
			}
		})
	}

	if _, err := WebSocketURL("ftp://printer.local"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
}
