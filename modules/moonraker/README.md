# Moonraker Go client

Standalone Go module for Moonraker's HTTP API and WebSocket status messages.
Requires Go 1.24 or newer. The only external dependency is `gorilla/websocket`.

The module currently lives in Filatrack's `modules/moonraker` directory, with
module path `github.com/yanefed/moonraker`. This is the intended repository path;
it has not been published there. Filatrack uses a local `replace` in `go.mod`.
The directory can be moved intact to a separate repository. After publishing a
version there, consumers can use `go get github.com/yanefed/moonraker@<version>`
and Filatrack can remove its local replacement.

## HTTP

```go
client := moonraker.Client{
    HTTPClient: &http.Client{Timeout: 15 * time.Second},
}
server := moonraker.Config{
    URL: "http://printer.local:7125",
    APIKey: os.Getenv("MOONRAKER_API_KEY"),
}

jobs, err := client.ListHistory(ctx, server, 25)
```

Operations:

- `Check`: check `/server/info` connectivity.
- `ListHistory`: fetch the latest jobs; a nonpositive limit fetches all pages.
- `ListHistoryPage` / `ListHistoryPageSince`: paginated history, optionally
  filtered by start time. The returned count is Moonraker's raw count; some
  versions report a page count rather than a total.
- `ListFiles`: files in the `gcodes` root.
- `FetchMetadata`: slicer metadata and the largest available thumbnail.
  Thumbnail download errors are best effort and do not discard metadata.
- `StartFile`, `Upload`, `DownloadFile`: print and transfer G-code files.

`Config.URL` accepts `http`, `https`, `ws`, and `wss`. API keys are sent using
`X-Api-Key`. Endpoint paths are rooted at the server host, matching the original
Filatrack client; reverse proxies under a URL path prefix are not supported.
A nil `HTTPClient` uses `http.DefaultClient`; callers should set timeouts or
context deadlines. Uploads currently buffer the multipart body in memory.

## WebSocket

```go
conn, err := (moonraker.WebSocketDialer{}).Dial(ctx, server)
if err != nil {
    return err
}
defer conn.Close()

if err := conn.WriteJSON(moonraker.SubscribeRequest()); err != nil {
    return err
}
for {
    var raw json.RawMessage
    if err := conn.ReadJSON(&raw); err != nil {
        return err
    }
    update, ok, err := moonraker.DecodeStatusUpdate(raw)
    if err != nil {
        return err
    }
    if ok {
        // Merge partial fields using HasState, HasFilename, and HasProgress.
        // RawPrintState and KlippyState preserve protocol values.
        _ = update
    }
}
```

The subscription covers `print_stats`, `virtual_sdcard`, `display_status`, and
`webhooks`. The decoder handles the initial snapshot and subsequent partial
notifications, and normalizes progress to 0–1. The dial context controls the
handshake; close the connection to interrupt reads. The library does not manage
reconnection, merge status updates, or infer completed print events.

Local printer IDs, filament inventory, print status mapping, thumbnail hashes
and disk caching belong to the consuming application. History statuses remain
strings so new Moonraker states do not require a library release.

## Tests

Run from this directory (root `go test ./...` skips nested modules):

```sh
go test -race ./...
go vet ./...
```
