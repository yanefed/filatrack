package integration_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/yanefed/filatrack/internal/domain"
	"github.com/yanefed/filatrack/internal/repository/sqlite"
	"github.com/yanefed/filatrack/internal/service"
)

func BenchmarkUIData(b *testing.B) {
	db, err := sqlite.New(filepath.Join(b.TempDir(), "ui.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		b.Fatal(err)
	}
	tx, err := db.Conn().Begin()
	if err != nil {
		b.Fatal(err)
	}
	for i := range 300 {
		if _, err := tx.Exec(`INSERT INTO spools(id,display_code,manufacturer,material,color,total_weight_g,diameter_mm,density_g_cm3) VALUES(?, ?, 'Maker','PLA','White',1000,1.75,1.24)`, fmt.Sprint(i), fmt.Sprint(i)); err != nil {
			b.Fatal(err)
		}
	}
	for i := range 5 {
		if _, err := tx.Exec(`INSERT INTO printers(id,name,kind,moonraker_url) VALUES(?, 'Printer','moonraker','http://printer')`, fmt.Sprint(i)); err != nil {
			b.Fatal(err)
		}
	}
	for i := range 500 {
		if _, err := tx.Exec(`INSERT INTO printer_models(printer_id,path) VALUES(?,?)`, fmt.Sprint(i%5), fmt.Sprintf("%d.gcode", i)); err != nil {
			b.Fatal(err)
		}
	}
	for i := range 5000 {
		if _, err := tx.Exec(`INSERT INTO print_jobs(id,spool_id,gcode_filename,status,estimated_weight_g,actual_weight_g) VALUES(?,?,'part.gcode','success',1,1)`, fmt.Sprint(i), fmt.Sprint(i%300)); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	var count atomic.Int64
	spoolRepo := countedSpools{sqlite.NewSpoolRepo(db), &count}
	dryingRepo := countedDryings{sqlite.NewDryingRepo(db), &count}
	settings := countedSettings{sqlite.NewSettingsRepo(db), &count}
	spools := service.NewSpoolService(spoolRepo, dryingRepo, settings)
	drying := service.NewDryingService(dryingRepo, settings)
	filter := domain.SpoolFilter{}
	ctx := context.Background()
	b.Run("PerCard", func(b *testing.B) {
		count.Store(0)
		b.ReportAllocs()
		for b.Loop() {
			items, err := spools.List(ctx, filter)
			if err != nil {
				b.Fatal(err)
			}
			for i := range items {
				if _, _, err := spools.GetWithRemaining(ctx, items[i].ID); err != nil {
					b.Fatal(err)
				}
				if _, err := drying.GetStatus(ctx, &items[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
		b.ReportMetric(float64(count.Load())/float64(b.N), "reads/op")
	})
	b.Run("Batched", func(b *testing.B) {
		count.Store(0)
		b.ReportAllocs()
		for b.Loop() {
			if _, err := spools.ListSummaries(ctx, filter); err != nil {
				b.Fatal(err)
			}
		}
		b.ReportMetric(float64(count.Load())/float64(b.N), "reads/op")
	})
	b.Run("Models500", func(b *testing.B) {
		repo := sqlite.NewPrinterModelRepo(db)
		b.ReportAllocs()
		for b.Loop() {
			if _, err := repo.List(ctx, nil); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("PrintHistory250", func(b *testing.B) {
		svc := service.NewPrintService(sqlite.NewPrintJobRepo(db), sqlite.NewSpoolRepo(db), nil)
		b.ReportAllocs()
		for b.Loop() {
			if _, err := svc.List(ctx, domain.PrintJobFilter{Limit: 250}); err != nil {
				b.Fatal(err)
			}
		}
	})

}
