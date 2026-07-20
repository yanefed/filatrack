package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"awesomeProject/internal/gcode"
)

var (
	stdout      io.Writer = os.Stdout
	stderr      io.Writer = os.Stderr
	exitProcess           = os.Exit
)

func main() {
	if code := run(os.Args, stdout, stderr); code != 0 {
		exitProcess(code)
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "Использование: gcodecheck <файл.gcode>")
		return 1
	}

	path := args[1]
	meta, err := gcode.ParseFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "Ошибка: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "=== Результаты парсинга G-code ===")
	fmt.Fprintf(stdout, "Файл:              %s\n", path)
	fmt.Fprintf(stdout, "Слайсер:           %s\n", valOrNA(meta.Slicer))
	fmt.Fprintf(stdout, "Модель:            %s\n", valOrNA(meta.ModelName))
	fmt.Fprintf(stdout, "Длина филамента:   %.2f мм (%.2f м)\n", meta.FilamentLengthMM, meta.FilamentLengthMM/1000)
	fmt.Fprintf(stdout, "Вес филамента:     %.2f г\n", meta.FilamentWeightG)
	fmt.Fprintf(stdout, "Время печати:      %s\n", formatDuration(meta.PrintDuration))
	fmt.Fprintf(stdout, "Температура сопла: %s\n", tempOrNA(meta.NozzleTempC))
	fmt.Fprintf(stdout, "Температура стола: %s\n", tempOrNA(meta.BedTempC))
	fmt.Fprintf(stdout, "Высота слоя:       %s\n", layerOrNA(meta.LayerHeight))

	if meta.FilamentLengthMM > 0 && meta.FilamentWeightG == 0 {
		fmt.Fprintln(stdout, "\n--- Расчёт веса по длине (PLA, 1.75мм, 1.24 г/см³) ---")
		w := gcode.WeightFromLength(meta.FilamentLengthMM, 1.75, 1.24)
		fmt.Fprintf(stdout, "Расчётный вес:     %.2f г\n", w)
	}
	return 0
}

func valOrNA(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func tempOrNA(t int) string {
	if t == 0 {
		return "—"
	}
	return fmt.Sprintf("%d °C", t)
}

func layerOrNA(h float64) string {
	if h == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f мм", h)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "—"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dч %dм %dс", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dм %dс", m, s)
	}
	return fmt.Sprintf("%dс", s)
}
