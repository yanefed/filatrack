package shared

import "testing"

func TestParseDecimal(t *testing.T) {
	for text, want := range map[string]float64{"1,75": 1.75, " 1.75 ": 1.75, "0": 0} {
		value, err := ParseDecimal(text)
		if err != nil || value != want {
			t.Errorf("%q: %v, %v", text, value, err)
		}
	}
	for _, text := range []string{"1,000.50", "1 000", "NaN", "Inf", "1,2,3", ""} {
		if _, err := ParseDecimal(text); err == nil {
			t.Errorf("accepted invalid number %q", text)
		}
	}
}
