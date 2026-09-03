package main

import "testing"

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		minor int64
		want  string
	}{
		{0, "₹0.00"},
		{1, "₹0.01"},
		{100, "₹1.00"},
		{1234, "₹12.34"},
		{12345, "₹123.45"},
		{123456, "₹1,234.56"},
		{1234567, "₹12,345.67"},
		{12345678, "₹1,23,456.78"},
		{100000000, "₹10,00,000.00"},
		{-500, "-₹5.00"},
		{-123456, "-₹1,234.56"},
	}
	for _, c := range cases {
		if got := formatMoney(c.minor); got != c.want {
			t.Errorf("formatMoney(%d) = %q, want %q", c.minor, got, c.want)
		}
	}
}

func TestGroupIndian(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"0", "0"},
		{"999", "999"},
		{"1000", "1,000"},
		{"12345", "12,345"},
		{"123456", "1,23,456"},
		{"1234567", "12,34,567"},
		{"12345678", "1,23,45,678"},
	}
	for _, c := range cases {
		if got := groupIndian(c.in); got != c.want {
			t.Errorf("groupIndian(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatPct(t *testing.T) {
	one := 0.912
	if got := formatPct(&one); got != "91%" {
		t.Errorf("formatPct(0.912) = %q, want 91%%", got)
	}
	if got := formatPct(nil); got != "—" {
		t.Errorf("formatPct(nil) = %q, want —", got)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "he…"},
		{"hello", 0, "…"},
		{"", 3, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestWrapText(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  []string
	}{
		{"short line", 20, []string{"short line"}},
		// word boundaries: never split a word when it fits the next line
		{"one two three four", 8, []string{"one two", "three", "four"}},
		// paragraph breaks are preserved
		{"para one\n\npara two", 40, []string{"para one", "", "para two"}},
		// lines are capped at width (indent space is added by the caller)
		{"abcdefghij", 5, []string{"abcde", "fghij"}},
		// a single token wider than width is hard-split, not dropped
		{"C7H3F12N8-token", 6, []string{"C7H3F1", "2N8-to", "ken"}},
		{"", 10, []string{}},
	}
	for _, c := range cases {
		got := wrapText(c.in, c.width)
		if len(got) != len(c.want) {
			t.Fatalf("wrapText(%q, %d) = %#v, want %#v", c.in, c.width, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("wrapText(%q, %d)[%d] = %q, want %q", c.in, c.width, i, got[i], c.want[i])
			}
		}
	}
}
