package domain

const DefaultCurrency = "RUB"

var Currencies = []string{
	DefaultCurrency,
	"USD",
	"EUR",
}

func ValidCurrency(currency string) bool {
	for _, supported := range Currencies {
		if currency == supported {
			return true
		}
	}
	return false
}
