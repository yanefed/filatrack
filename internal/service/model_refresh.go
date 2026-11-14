package service

import (
	"fmt"

	"github.com/yanefed/filatrack/internal/domain"
)

type ModelRefreshMode int

const (
	ModelRefreshChanges ModelRefreshMode = iota
	ModelRefreshFull
)

type ModelRefreshFailure struct {
	PrinterID   string
	PrinterName string
	Path        string
	Err         error
}

func (f ModelRefreshFailure) Error() string {
	name := f.PrinterName
	if name == "" {
		name = f.PrinterID
	}
	if f.Path == "" {
		return fmt.Sprintf("%s: %v", name, f.Err)
	}
	return fmt.Sprintf("%s / %s: %v", name, f.Path, f.Err)
}

func (f ModelRefreshFailure) Unwrap() error { return f.Err }

type ModelRefreshResult struct {
	Models   []domain.PrinterModelFile
	Failures []ModelRefreshFailure
}
