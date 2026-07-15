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
}
