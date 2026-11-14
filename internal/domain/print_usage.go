package domain

// PrintUsage describes persisted consumption independently of a print's identity.
type PrintUsage struct {
	EstimatedWeightG  float64
	ActualWeightG     float64
	LengthMM          float64
	CompletionPercent int
	Measured          bool
}

func (j PrintJob) Usage() PrintUsage {
	return PrintUsage{
		EstimatedWeightG:  j.EstimatedWeightG,
		ActualWeightG:     j.ActualWeightG,
		LengthMM:          j.EstimatedLengthMM,
		CompletionPercent: j.CompletionPercent,
		Measured:          j.UsageMeasured,
	}
}

// WithSpool recalculates weight from consumed length without changing its provenance.
func (u PrintUsage) WithSpool(spool Spool, status PrintStatus) PrintUsage {
	u = u.withSpoolWeight(spool, status)
	if u.LengthMM > 0 && u.CompletionPercent <= 0 && u.EstimatedWeightG > 0 {
		u.CompletionPercent = usageCompletion(u.ActualWeightG, u.EstimatedWeightG)
	}
	return u
}

func (u PrintUsage) withSpoolWeight(spool Spool, status PrintStatus) PrintUsage {
	if u.LengthMM <= 0 {
		return u
	}
	u.ActualWeightG = WeightFromLength(u.LengthMM, spool.DiameterMM, spool.DensityGCM3)
	if u.EstimatedWeightG == 0 && status == PrintStatusSuccess {
		u.EstimatedWeightG = u.ActualWeightG
	}
	return u
}

func (u PrintUsage) WithHistory(item PrinterHistoryJob, spool *Spool, localStatus PrintStatus) PrintUsage {
	if item.EstimatedWeightG > 0 {
		u.EstimatedWeightG = item.EstimatedWeightG
	}
	if item.FilamentUsedMM > 0 {
		u.LengthMM = item.FilamentUsedMM
	}
	if spool != nil {
		u = u.withSpoolWeight(*spool, localStatus)
		u.Measured = u.Measured || item.FilamentUsedMM > 0
	}
	if status, finished := item.PrintStatus(); finished {
		if status == PrintStatusSuccess {
			u.CompletionPercent = 100
			if u.ActualWeightG == 0 && u.EstimatedWeightG > 0 {
				u.ActualWeightG = u.EstimatedWeightG
			}
		} else if u.EstimatedWeightG > 0 && u.ActualWeightG > 0 {
			u.CompletionPercent = usageCompletion(u.ActualWeightG, u.EstimatedWeightG)
		}
	}
	return u
}

func (u PrintUsage) Apply(job *PrintJob) {
	job.EstimatedWeightG, job.ActualWeightG, job.EstimatedLengthMM = u.EstimatedWeightG, u.ActualWeightG, u.LengthMM
	job.CompletionPercent, job.UsageMeasured = u.CompletionPercent, u.Measured
}

func usageCompletion(actual, estimated float64) int {
	return min(max(int(actual/estimated*100), 0), 100)
}
