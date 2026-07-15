package main

import (
	"fmt"
	"os"
	"time"

	"awesomeProject/internal/gcode"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Использование: gcodecheck <файл.gcode>\n")
		os.Exit(1)
	}

	path := os.Args[1]
	meta, err := gcode.ParseFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Результаты парсинга G-code ===")
	fmt.Printf("Файл:              %s\n", path)
	fmt.Printf("Слайсер:           %s\n", valOrNA(meta.Slicer))
	fmt.Printf("Модель:            %s\n", valOrNA(meta.ModelName))
	fmt.Printf("Длина филамента:   %.2f мм (%.2f м)\n", meta.FilamentLengthMM, meta.FilamentLengthMM/1000)
	fmt.Printf("Вес филамента:     %.2f г\n", meta.FilamentWeightG)
	fmt.Printf("Время печати:      %s\n", formatDuration(meta.PrintDuration))
	fmt.Printf("Температура сопла: %s\n", tempOrNA(meta.NozzleTempC))
	fmt.Printf("Температура стола: %s\n", tempOrNA(meta.BedTempC))
	fmt.Printf("Высота слоя:       %s\n", layerOrNA(meta.LayerHeight))

	if meta.FilamentLengthMM > 0 && meta.FilamentWeightG == 0 {
		fmt.Println("\n--- Расчёт веса по длине (PLA, 1.75мм, 1.24 г/см³) ---")
		w := gcode.WeightFromLength(meta.FilamentLengthMM, 1.75, 1.24)
		fmt.Printf("Расчётный вес:     %.2f г\n", w)
	}
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
