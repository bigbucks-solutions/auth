package actions

import "testing"

// An organization's country decides which priced plan it is billed under
// (subscriptions.CurrencyConfig), so once it has one, a settings edit must not
// be able to move it. Filling in a blank one is not a move, and neither is the
// settings form echoing the current value back in a whole-record PUT.
func TestCountryMoveRefused(t *testing.T) {
	tests := []struct {
		name      string
		existing  string
		requested string
		refused   bool
	}{
		{name: "moving to another country", existing: "IN", requested: "AE", refused: true},
		{name: "echoing the stored value back", existing: "IN", requested: "IN"},
		{name: "omitted by the client", existing: "IN", requested: ""},
		{name: "filling in a blank country", existing: "", requested: "IN"},
		{name: "blank on both sides", existing: "", requested: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countryMoveRefused(test.existing, test.requested); got != test.refused {
				t.Fatalf("countryMoveRefused(%q, %q) = %v, want %v",
					test.existing, test.requested, got, test.refused)
			}
		})
	}
}
