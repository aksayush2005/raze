package money

import (
	"errors"
	"fmt"
	"strings"
)

// Money represents a monetary amount as integer minor units (e.g. paise for INR).
// Financial truth is never stored or computed as floating point.
type Money int64

// NewMinor constructs a Money value from raw minor units.
func NewMinor(minor int64) Money { return Money(minor) }

// Minor returns the raw integer minor-unit value.
func (m Money) Minor() int64 { return int64(m) }

// ParseDecimal parses a decimal string such as "100.00" into minor units.
// Up to two decimal places are accepted; more are rejected to avoid silent truncation.
func ParseDecimal(s string) (Money, error) {
	neg := false
	body := strings.TrimSpace(s)
	if strings.HasPrefix(body, "-") {
		neg = true
		body = strings.TrimPrefix(body, "-")
	}
	if body == "" {
		return 0, errors.New("money: empty amount")
	}
	parts := strings.Split(body, ".")
	if len(parts) > 2 {
		return 0, errors.New("money: invalid decimal")
	}
	intPart := parts[0]
	fracPart := "00"
	if len(parts) == 2 {
		fracPart = parts[1]
		if fracPart == "" {
			fracPart = "00"
		}
	}
	if len(fracPart) > 2 {
		return 0, errors.New("money: more than 2 decimal places")
	}
	for len(fracPart) < 2 {
		fracPart += "0"
	}
	if intPart == "" {
		intPart = "0"
	}
	var v int64
	for _, c := range intPart + fracPart {
		if c < '0' || c > '9' {
			return 0, errors.New("money: non-numeric character")
		}
		v = v*10 + int64(c-'0')
	}
	if neg {
		v = -v
	}
	return Money(v), nil
}

// String formats the amount as a decimal string, e.g. "100.00".
func (m Money) String() string {
	neg := m.Minor() < 0
	abs := m.Minor()
	if neg {
		abs = -abs
	}
	s := fmt.Sprintf("%d.%02d", abs/100, abs%100)
	if neg {
		s = "-" + s
	}
	return s
}

func (m Money) Add(o Money) Money { return m + o }
func (m Money) Sub(o Money) Money { return m - o }
func (m Money) Neg() Money        { return -m }
func (m Money) IsZero() bool      { return m == 0 }
