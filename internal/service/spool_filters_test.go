package service_test

import (
	"awesomeProject/internal/domain"
	"awesomeProject/internal/repository/sqlite"
	"awesomeProject/internal/service"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSpoolFiltering(t *testing.T) {
	db, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	svc := service.NewSpoolService(sqlite.NewSpoolRepo(db), nil, nil)
	ctx := t.Context()
	for _, spool := range []domain.Spool{
		{Manufacturer: "B", Material: domain.MaterialPLA},
		{Manufacturer: "A", Material: domain.MaterialPLA},
		{Manufacturer: "A", Material: "PLA+"},
		{Manufacturer: "A", Material: "PLA+"},
		{Manufacturer: "", Material: domain.MaterialPETG},
		{Manufacturer: "Archived", Material: "Archived", Archived: true},
	} {
		spool.TotalWeightG = 1000
		if err := svc.Create(ctx, &spool); err != nil {
			t.Fatal(err)
		}
	}
	options, err := svc.FilterOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(options.Manufacturers, []string{"", "A", "B"}) ||
		!reflect.DeepEqual(options.Materials, []domain.Material{domain.MaterialPETG, domain.MaterialPLA, "PLA+"}) {
		t.Fatalf("unexpected filter options: %+v", options)
	}
	active := false
	manufacturer := "A"
	material := domain.Material("PLA+")
	unknown := "Missing"
	empty := ""
	for _, test := range []struct {
		name   string
		filter domain.SpoolFilter
		want   int
	}{
		{"all active", domain.SpoolFilter{Archived: &active}, 5},
		{"manufacturer", domain.SpoolFilter{Archived: &active, Manufacturer: &manufacturer}, 3},
		{"custom material", domain.SpoolFilter{Archived: &active, Material: &material}, 2},
		{"combined", domain.SpoolFilter{Archived: &active, Manufacturer: &manufacturer, Material: &material}, 2},
		{"no matches", domain.SpoolFilter{Archived: &active, Manufacturer: &unknown, Material: &material}, 0},
		{"unspecified manufacturer", domain.SpoolFilter{Archived: &active, Manufacturer: &empty}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			spools, err := svc.List(ctx, test.filter)
			if err != nil || len(spools) != test.want {
				t.Fatalf("List() = %d spools, %v; want %d", len(spools), err, test.want)
			}
			for _, spool := range spools {
				if spool.Archived || (test.filter.Manufacturer != nil && spool.Manufacturer != *test.filter.Manufacturer) ||
					(test.filter.Material != nil && spool.Material != *test.filter.Material) {
					t.Fatalf("spool does not match filter: %+v", spool)
				}
			}
		})
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FilterOptions(ctx); err == nil {
		t.Fatal("expected repository error to be propagated")
	}
}
