package ui

import (
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/yanefed/filatrack/internal/ui/widgets"
)

// startMemoryDiagnostics logs Go heap and process RSS when explicitly enabled.
func startMemoryDiagnostics() glib.SourceHandle {
	if os.Getenv("FILATRACK_MEMDEBUG") != "1" {
		return 0
	}
	return glib.TimeoutSecondsAdd(5, func() bool {
		// A forced collection makes the snapshot distinguish live Go objects
		// from short-lived allocations. This runs only in opt-in debug mode.
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		attrs := []any{
			"heap_alloc_bytes", stats.HeapAlloc,
			"heap_inuse_bytes", stats.HeapInuse,
			"heap_objects", stats.HeapObjects,
			"gc_cycles", stats.NumGC,
			"preview_textures", widgets.PreviewTextureCacheSize(),
		}
		if memory, ok := processMemoryBytes(); ok {
			attrs = append(attrs,
				"rss_bytes", memory["VmRSS"],
				"rss_anon_bytes", memory["RssAnon"],
				"rss_file_bytes", memory["RssFile"],
			)
		}
		slog.Info("memory snapshot", attrs...)
		return true
	})
}

func processMemoryBytes() (map[string]uint64, bool) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return nil, false
	}
	memory := make(map[string]uint64, 3)
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		switch name {
		case "VmRSS", "RssAnon", "RssFile":
		default:
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		memory[name] = kib * 1024
	}
	return memory, len(memory) == 3
}
