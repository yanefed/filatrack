package domain

import "time"

type GcodeMetadata struct {
	Filename         string
	ModelName        string
	Slicer           string
	FilamentLengthMM float64
	FilamentWeightG  float64
	PrintDuration    time.Duration
	NozzleTempC      int
	BedTempC         int
	LayerHeight      float64
	PreviewImagePath string
	PreviewImageHash string
	PreviewImageData []byte
}

func (m *GcodeMetadata) MergeMissingGcodeMetadata(fallback *GcodeMetadata) {
	if m == nil || fallback == nil {
		return
	}
	if m.FilamentWeightG <= 0 {
		m.FilamentWeightG = fallback.FilamentWeightG
	}
	if m.FilamentLengthMM <= 0 {
		m.FilamentLengthMM = fallback.FilamentLengthMM
	}
	if m.PrintDuration <= 0 {
		m.PrintDuration = fallback.PrintDuration
	}
	if m.Slicer == "" {
		m.Slicer = fallback.Slicer
	}
	if m.PreviewImagePath == "" {
		m.PreviewImagePath = fallback.PreviewImagePath
	}
	if m.PreviewImageHash == "" {
		m.PreviewImageHash = fallback.PreviewImageHash
	}
	if len(m.PreviewImageData) == 0 {
		m.PreviewImageData = fallback.PreviewImageData
	}
}
