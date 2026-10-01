// Package words turns numbers into the phrases the jokes are made of.
package words

import (
	"fmt"
	"strings"
	"time"
)

// Plural returns "1 file" or "3 files".
func Plural(n int64, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// Duration renders the two most significant units, e.g. "2 days 3 hours",
// "41 minutes", "12 seconds".
func Duration(d time.Duration) string {
	return durationParts(d, 2)
}

// LongDuration renders days, hours and minutes, skipping zero units, e.g.
// "16 days 4 hours 51 minutes".
func LongDuration(d time.Duration) string {
	if d < time.Minute {
		return "0 minutes"
	}
	return durationParts(d.Truncate(time.Minute), 3)
}

func durationParts(d time.Duration, max int) string {
	if d < 0 {
		d = 0
	}
	secs := int64(d / time.Second)
	units := []struct {
		name string
		size int64
	}{{"day", 86400}, {"hour", 3600}, {"minute", 60}, {"second", 1}}
	var parts []string
	started := false
	for _, u := range units {
		n := secs / u.size
		secs %= u.size
		if n == 0 && !started {
			continue
		}
		started = true
		if n > 0 {
			parts = append(parts, Plural(n, u.name))
		}
		max--
		if max == 0 {
			break
		}
	}
	if len(parts) == 0 {
		return "0 seconds"
	}
	return strings.Join(parts, " ")
}

// Bytes renders a size as "840 KB", "612 MB", "1.4 GB" (1024-based, which
// is what Finder's neighbours in the terminal use).
func Bytes(n int64) string {
	const k = 1024
	switch {
	case n < k:
		return Plural(n, "byte")
	case n < k*k:
		return fmt.Sprintf("%d KB", n/k)
	case n < k*k*k:
		return fmt.Sprintf("%d MB", n/(k*k))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	}
}

var ones = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
	"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
var tens = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}

// Number spells out n for 0..9999 ("sixteen", "one hundred and two") and
// falls back to digits beyond that.
func Number(n int64) string {
	switch {
	case n < 0 || n > 9999:
		return fmt.Sprintf("%d", n)
	case n < 20:
		return ones[n]
	case n < 100:
		if n%10 == 0 {
			return tens[n/10]
		}
		return tens[n/10] + "-" + ones[n%10]
	case n < 1000:
		s := ones[n/100] + " hundred"
		if n%100 != 0 {
			s += " and " + Number(n%100)
		}
		return s
	default:
		s := ones[n/1000] + " thousand"
		if r := n % 1000; r != 0 {
			if r < 100 {
				s += " and " + Number(r)
			} else {
				s += " " + Number(r)
			}
		}
		return s
	}
}

// Capitalize upper-cases the first letter.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var ordinals = []string{"zeroth", "first", "second", "third", "fourth", "fifth", "sixth",
	"seventh", "eighth", "ninth", "tenth", "eleventh", "twelfth"}

// Ordinal spells "third" for 3; beyond twelve it uses "13th".
func Ordinal(n int) string {
	if n >= 0 && n < len(ordinals) {
		return ordinals[n]
	}
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// Truncate shortens s to max runes with a trailing ellipsis.
func Truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// Wrap breaks text into lines no longer than width, on spaces.
func Wrap(text string, width int) []string {
	var lines []string
	var cur string
	for _, w := range strings.Fields(text) {
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
