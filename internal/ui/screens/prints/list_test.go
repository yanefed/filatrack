package prints

import (
	"context"
	"fmt"
	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/i18n"
	"runtime"
	"testing"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestPrintsListIsVirtualized(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("GTK windows cannot be presented from Go's test goroutine on macOS")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if !gtk.InitCheck() {
		t.Skip("GTK display is not available")
	}
	const count = 1000
	jobs := make([]domain.PrintJob, count)
	for i := range jobs {
		jobs[i] = domain.PrintJob{ID: fmt.Sprintf("job-%d", i), ModelName: fmt.Sprintf("job-%d", i)}
	}
	view, update := newVirtualPrintsList(i18n.L(), context.Background(), nil, gtk.NewLabel(""))
	update(jobs, nil)
	window := gtk.NewWindow()
	window.SetDefaultSize(800, 600)
	scrolled := gtk.NewScrolledWindow()
	scrolled.SetChild(view)
	window.SetChild(scrolled)
	window.Present()
	mainContext := glib.MainContextDefault()
	for range 20 {
		mainContext.Iteration(false)
	}
	if widgets := printDescendantCount(view); widgets >= count*3 {
		t.Fatalf("prints view created %d widgets for %d records", widgets, count)
	}
	window.Destroy()
}

func TestPrintCostText(t *testing.T) {
	translations := i18n.T{PrintCostFmt: "Cost: %s%.2f %s", CostNotSet: "not set"}
	for _, tc := range []struct {
		job  domain.PrintJob
		want string
	}{
		{domain.PrintJob{}, "not set"},
		{domain.PrintJob{ActualWeightG: 4, PricePerGram: 3, CostCurrency: "EUR", UsageMeasured: true}, "Cost: 12.00 EUR"},
		{domain.PrintJob{ActualWeightG: 4, PricePerGram: 3, CostCurrency: "EUR"}, "Cost: ≈12.00 EUR"},
	} {
		if got := printCostText(translations, tc.job); got != tc.want {
			t.Errorf("printCostText(%+v) = %q, want %q", tc.job, got, tc.want)
		}
	}
}

func printDescendantCount(root gtk.Widgetter) int {
	count := 0
	for child := gtk.BaseWidget(root).FirstChild(); child != nil; child = gtk.BaseWidget(child).NextSibling() {
		count++
		count += printDescendantCount(child)
	}
	return count
}
