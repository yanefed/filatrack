package domain

import "slices"

const DefaultCurrency = "RUB"

var Currencies = []string{
	DefaultCurrency,
	"USD",
	"EUR",
}

func ValidCurrency(currency string) bool {
	return slices.Contains(Currencies, currency)
}
