package domain

import "testing"

func TestCurrency(t *testing.T) {
	if DefaultCurrency != "RUB" || !ValidCurrency(DefaultCurrency) {
		t.Fatalf("invalid default currency %q", DefaultCurrency)
	}
	for _, currency := range Currencies {
		if !ValidCurrency(currency) {
			t.Errorf("listed currency %q is invalid", currency)
		}
	}
	for _, currency := range []string{"", "rub", " RUB", "BTC"} {
		if ValidCurrency(currency) {
			t.Errorf("unsupported currency %q is valid", currency)
		}
	}
}
