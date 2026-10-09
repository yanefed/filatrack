package service

import "github.com/yanefed/filatrack/internal/domain"

// SpoolFilterOptions содержит доступные значения фильтров активных катушек.
type SpoolFilterOptions struct {
	Manufacturers []string
	Materials     []domain.Material
}
