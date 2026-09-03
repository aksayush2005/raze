package money

import "testing"

func TestParseDecimal(t *testing.T) {
	cases := map[string]int64{
		"100.00": 10000,
		"0.01":   1,
		"1":      100,
		"1.5":    150,
		"-25.99": -2599,
		"1000":   100000,
	}
	for in, want := range cases {
		got, err := ParseDecimal(in)
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", in, err)
		}
		if got.Minor() != want {
			t.Errorf("ParseDecimal(%q) = %d, want %d", in, got.Minor(), want)
		}
	}
}

func TestParseDecimalRejectsMoreThanTwoPlaces(t *testing.T) {
	if _, err := ParseDecimal("1.005"); err == nil {
		t.Fatal("expected error for >2 decimal places")
	}
	if _, err := ParseDecimal("abc"); err == nil {
		t.Fatal("expected error for non-numeric")
	}
}

func TestString(t *testing.T) {
	cases := map[int64]string{
		10000: "100.00",
		-2599: "-25.99",
		5:     "0.05",
	}
	for in, want := range cases {
		if got := NewMinor(in).String(); got != want {
			t.Errorf("String(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	a := NewMinor(10000)
	b := NewMinor(250)
	if got := a.Add(b); got.Minor() != 10250 {
		t.Errorf("add = %d", got.Minor())
	}
	if got := a.Sub(b); got.Minor() != 9750 {
		t.Errorf("sub = %d", got.Minor())
	}
}
