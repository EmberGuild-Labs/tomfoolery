package words

import (
	"testing"
	"time"
)

func TestWords(t *testing.T) {
	eq := func(got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
	eq(Duration(51*time.Hour+3*time.Minute), "2 days 3 hours")
	eq(Duration(41*time.Minute+5*time.Second), "41 minutes 5 seconds")
	eq(Duration(24*time.Hour+30*time.Second), "1 day")
	eq(Duration(0), "0 seconds")
	eq(LongDuration(16*24*time.Hour+4*time.Hour+51*time.Minute+30*time.Second), "16 days 4 hours 51 minutes")
	eq(LongDuration(24*time.Hour+3*time.Minute), "1 day 3 minutes")
	eq(LongDuration(30*time.Second), "0 minutes")
	eq(Bytes(612*1024*1024), "612 MB")
	eq(Bytes(1500*1024*1024), "1.5 GB")
	eq(Number(16), "sixteen")
	eq(Number(42), "forty-two")
	eq(Number(365), "three hundred and sixty-five")
	eq(Number(1001), "one thousand and one")
	eq(Ordinal(3), "third")
	eq(Ordinal(23), "23rd")
	eq(Ordinal(13), "13th")
	eq(Truncate("abcdef", 4), "abc…")
}
