package gcode

import (
	"awesomeProject/internal/domain"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrUnsupportedModelFile indicates that the input is a 3D model file, not sliced G-code.
var ErrUnsupportedModelFile = errors.New("unsupported 3D model file")

// Общие регулярные выражения для извлечения метаданных из комментариев G-code.
var (
	// Длина филамента: "filament used [mm] = 5000" или "filament used = 5.0m" или "Filament used: 5.0m"
	reFilamentUsedM  = regexp.MustCompile(`(?i)filament\s+used\s*[\[:=]?\s*([\d.]+)\s*m\b`)
	reFilamentUsedMM = regexp.MustCompile(`(?i)filament\s+used\s*\[mm\]\s*=\s*([\d.]+)`)

	// Вес филамента: "filament used [g] = 20.5" или "Filament weight = 20.5" или "filament_weight = 20.5"
	reFilamentWeightG    = regexp.MustCompile(`(?i)filament\s+used\s*\[g\]\s*=\s*([\d.]+)`)
	reFilamentWeightAlt  = regexp.MustCompile(`(?i)filament[\s_]weight\s*=\s*([\d.]+)`)
	reFilamentUsedGCura  = regexp.MustCompile(`(?i)filament\s+used\s*:\s*([\d.]+)\s*g`)
	reFilamentCostWeight = regexp.MustCompile(`(?i)filament\s+weight\s*=\s*([\d.]+)`)

	// Время печати
	rePrintTimeEst   = regexp.MustCompile(`(?i)estimated\s+printing\s+time.*?=\s*(.+)`)
	rePrintTimeCura  = regexp.MustCompile(`(?i)print\s+time[:\s]*=?\s*(.+)`)
	reTimeSeconds    = regexp.MustCompile(`(?i)time[:\s]*=?\s*(\d+)\s*s`)
	rePrintTimeBambu = regexp.MustCompile(`(?i)total\s+estimated\s+time.*?=\s*(.+)`)

	// Температуры
	reNozzleTemp    = regexp.MustCompile(`(?i)nozzle_temperature\s*=\s*(\d+)`)
	reNozzleTempAlt = regexp.MustCompile(`(?i)temperature\s*=\s*(\d+)`)
	reBedTemp       = regexp.MustCompile(`(?i)bed_temperature\s*=\s*(\d+)`)

	// Высота слоя
	reLayerHeight    = regexp.MustCompile(`(?i)layer_height\s*=\s*([\d.]+)`)
	reLayerHeightAlt = regexp.MustCompile(`(?i)layer\s+height[:\s]*=?\s*([\d.]+)`)

	// Имя модели / файла
	reModelNamePrusa = regexp.MustCompile(`(?i);\s*model\s*(?:name)?\s*[:=]\s*(.+)`)
	reInputFile      = regexp.MustCompile(`(?i);\s*input[\s_]file(?:name)?\s*[:=]\s*(.+)`)

	// Слайсер
	reGeneratedBy  = regexp.MustCompile(`(?i)generated\s+(?:by|with)\s+(.+)`)
	reSlicerPE     = regexp.MustCompile(`(?i);\s*slicer\s*[:=]\s*(.+)`)
	reSlicerEngine = regexp.MustCompile(`(?i);\s*slicer_version\s*[:=]\s*(.+)`)

	// Cura-специфичные
	reCuraTime  = regexp.MustCompile(`(?i);\s*TIME\s*:\s*(\d+)`)
	reCuraLayer = regexp.MustCompile(`(?i);\s*Layer\s+height\s*:\s*([\d.]+)`)

	// Длительность в формате "1h 30m 5s" или "1d 2h 30m"
	reDurDays = regexp.MustCompile(`(\d+)\s*d`)
	reDurHrs  = regexp.MustCompile(`(\d+)\s*h`)
	reDurMin  = regexp.MustCompile(`(\d+)\s*m(?:in)?`)
	reDurSec  = regexp.MustCompile(`(\d+)\s*s`)
)

// ParseFile открывает файл и парсит метаданные G-code.
func ParseFile(path string) (*domain.GcodeMetadata, error) {
	if isUnsupportedModelFile(path) {
		return nil, fmt.Errorf("%w: %s contains model geometry, not filament usage; slice it to G-code first", ErrUnsupportedModelFile, filepath.Ext(path))
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening gcode file: %w", err)
	}
	defer f.Close()

	meta, err := Parse(f)
	if err != nil {
		return nil, err
	}
	meta.Filename = path
	return meta, nil
}

func isUnsupportedModelFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".stl", ".obj", ".3mf":
		return true
	default:
		return false
	}
}

// Parse читает G-code из io.Reader и извлекает метаданные из комментариев.
func Parse(r io.Reader) (*domain.GcodeMetadata, error) {
	meta := &domain.GcodeMetadata{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, ";") {
			continue
		}
		parseCommentLine(trimmed, meta)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning gcode: %w", err)
	}

	normalizeSlicer(meta)
	return meta, nil
}

func parseCommentLine(line string, meta *domain.GcodeMetadata) {
	parseFilament(line, meta)
	parsePrintTime(line, meta)
	parseTemperatures(line, meta)
	parseLayerHeight(line, meta)
	parseModelName(line, meta)
	parseSlicerInfo(line, meta)
}

func parseFilament(line string, meta *domain.GcodeMetadata) {
	// Длина в мм (PrusaSlicer/OrcaSlicer: "filament used [mm] = 5000.00")
	if m := reFilamentUsedMM.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.FilamentLengthMM = v
			return
		}
	}

	// Длина в метрах (PrusaSlicer: "filament used = 5.0m", Cura: "Filament used: 5.0m")
	if m := reFilamentUsedM.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.FilamentLengthMM = v * 1000
			return
		}
	}

	// Вес в граммах: "filament used [g] = 20.5"
	if m := reFilamentWeightG.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.FilamentWeightG = v
			return
		}
	}

	// Вес: "filament weight = 20.5" или "filament_weight = 20.5"
	if m := reFilamentWeightAlt.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.FilamentWeightG = v
			return
		}
	}

	// Cura: "Filament used: 20.5g"
	if m := reFilamentUsedGCura.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.FilamentWeightG = v
			return
		}
	}

	// Общий fallback для веса
	if m := reFilamentCostWeight.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && meta.FilamentWeightG == 0 {
			meta.FilamentWeightG = v
		}
	}
}

func parsePrintTime(line string, meta *domain.GcodeMetadata) {
	if meta.PrintDuration > 0 {
		return
	}

	// PrusaSlicer/OrcaSlicer: "estimated printing time = 1h 30m 5s"
	if m := rePrintTimeEst.FindStringSubmatch(line); m != nil {
		meta.PrintDuration = parseDuration(m[1])
		return
	}

	// BambuStudio: "total estimated time = ..."
	if m := rePrintTimeBambu.FindStringSubmatch(line); m != nil {
		meta.PrintDuration = parseDuration(m[1])
		return
	}

	// Cura: ";TIME:3600" (секунды)
	if m := reCuraTime.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			meta.PrintDuration = time.Duration(v) * time.Second
			return
		}
	}

	// Общий: "print time: 1h 30m" или "print time = 1h 30m"
	if m := rePrintTimeCura.FindStringSubmatch(line); m != nil {
		if d := parseDuration(m[1]); d > 0 {
			meta.PrintDuration = d
			return
		}
	}

	// Время в секундах: "time = 3600s"
	if m := reTimeSeconds.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			meta.PrintDuration = time.Duration(v) * time.Second
		}
	}
}

func parseTemperatures(line string, meta *domain.GcodeMetadata) {
	if m := reNozzleTemp.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			meta.NozzleTempC = v
		}
		return
	}

	if meta.NozzleTempC == 0 {
		if m := reNozzleTempAlt.FindStringSubmatch(line); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil {
				meta.NozzleTempC = v
			}
			return
		}
	}

	if m := reBedTemp.FindStringSubmatch(line); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil {
			meta.BedTempC = v
		}
	}
}

func parseLayerHeight(line string, meta *domain.GcodeMetadata) {
	if meta.LayerHeight > 0 {
		return
	}

	if m := reLayerHeight.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.LayerHeight = v
			return
		}
	}

	if m := reLayerHeightAlt.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.LayerHeight = v
			return
		}
	}

	if m := reCuraLayer.FindStringSubmatch(line); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			meta.LayerHeight = v
		}
	}
}

func parseModelName(line string, meta *domain.GcodeMetadata) {
	if meta.ModelName != "" {
		return
	}

	if m := reModelNamePrusa.FindStringSubmatch(line); m != nil {
		meta.ModelName = strings.TrimSpace(m[1])
		return
	}

	if m := reInputFile.FindStringSubmatch(line); m != nil {
		name := strings.TrimSpace(m[1])
		// Убираем расширение и путь
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if idx := strings.LastIndex(name, "\\"); idx >= 0 {
			name = name[idx+1:]
		}
		if strings.HasSuffix(strings.ToLower(name), ".stl") ||
			strings.HasSuffix(strings.ToLower(name), ".3mf") ||
			strings.HasSuffix(strings.ToLower(name), ".obj") {
			name = name[:len(name)-4]
		}
		meta.ModelName = name
	}
}

func parseSlicerInfo(line string, meta *domain.GcodeMetadata) {
	if meta.Slicer != "" {
		return
	}

	if m := reGeneratedBy.FindStringSubmatch(line); m != nil {
		meta.Slicer = strings.TrimSpace(m[1])
		return
	}

	if m := reSlicerPE.FindStringSubmatch(line); m != nil {
		meta.Slicer = strings.TrimSpace(m[1])
		return
	}

	if m := reSlicerEngine.FindStringSubmatch(line); m != nil {
		meta.Slicer = strings.TrimSpace(m[1])
	}
}

// normalizeSlicer приводит имя слайсера к каноническому виду.
func normalizeSlicer(meta *domain.GcodeMetadata) {
	s := strings.ToLower(meta.Slicer)
	switch {
	case strings.Contains(s, "prusaslicer"):
		meta.Slicer = "PrusaSlicer"
	case strings.Contains(s, "cura"):
		meta.Slicer = "Cura"
	case strings.Contains(s, "orcaslicer"):
		meta.Slicer = "OrcaSlicer"
	case strings.Contains(s, "bambu studio"), strings.Contains(s, "bambustudio"):
		meta.Slicer = "BambuStudio"
	case strings.Contains(s, "slic3r"):
		meta.Slicer = "Slic3r"
	case strings.Contains(s, "simplify3d"):
		meta.Slicer = "Simplify3D"
	case strings.Contains(s, "ideamaker"):
		meta.Slicer = "IdeaMaker"
	}
}

// parseDuration парсит строку длительности вида "1d 2h 30m 5s" в time.Duration.
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	var total time.Duration

	if m := reDurDays.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += time.Duration(v) * 24 * time.Hour
	}
	if m := reDurHrs.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += time.Duration(v) * time.Hour
	}
	if m := reDurMin.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += time.Duration(v) * time.Minute
	}
	if m := reDurSec.FindStringSubmatch(s); m != nil {
		v, _ := strconv.Atoi(m[1])
		total += time.Duration(v) * time.Second
	}
	return total
}

// WeightFromLength рассчитывает вес филамента по длине, диаметру и плотности.
func WeightFromLength(lengthMM, diameterMM, densityGCM3 float64) float64 {
	radiusCM := diameterMM / 2 / 10
	lengthCM := lengthMM / 10
	volumeCM3 := math.Pi * radiusCM * radiusCM * lengthCM
	return volumeCM3 * densityGCM3
}
