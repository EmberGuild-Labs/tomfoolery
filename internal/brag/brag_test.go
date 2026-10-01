package brag

import (
	"strings"
	"testing"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

func TestParseBootTime(t *testing.T) {
	sec, err := ParseBootTime("{ sec = 1789000000, usec = 123456 } Mon Sep 14 09:12:00 2026\n")
	if err != nil || sec != 1789000000 {
		t.Fatalf("got %d %v", sec, err)
	}
	if _, err := ParseBootTime("garbage"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSameBoot(t *testing.T) {
	cases := []struct {
		a, b int64
		want bool
	}{
		{1000, 1000, true},
		{1000, 1120, true}, // clock correction within tolerance
		{1120, 1000, true},
		{1000, 1121, false}, // a real reboot
		{1000, 90000, false},
	}
	for _, c := range cases {
		if got := SameBoot(c.a, c.b); got != c.want {
			t.Errorf("SameBoot(%d,%d)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestFuneral(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	boot1 := int64(1_000_000)
	now := time.Unix(boot1+3*86400, 0)
	if err := PrepareFuneral(boot1, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := TakeFuneral(); ok {
		t.Fatal("no funeral on the first ever boot")
	}
	// Same boot, clock nudged: still no funeral.
	PrepareFuneral(boot1+60, now)
	if _, ok := TakeFuneral(); ok {
		t.Fatal("clock correction must not count as a reboot")
	}
	// Reboot.
	boot2 := boot1 + 4*86400
	PrepareFuneral(boot2, time.Unix(boot2+60, 0))
	f, ok := TakeFuneral()
	if !ok || f.Streak != 3*24*time.Hour || !f.NewRecord {
		t.Fatalf("funeral %+v %v", f, ok)
	}
	if Record() != 3*24*time.Hour {
		t.Fatalf("record %v", Record())
	}
	if _, ok := TakeFuneral(); ok {
		t.Fatal("funeral should be consumed")
	}
	if v, _ := state.ReadInts(state.LastSeen()); v[0] != boot2 {
		t.Fatalf("last-seen not reset to new boot: %v", v)
	}
}

func TestLine(t *testing.T) {
	ms := []int{1, 3, 7, 14, 30, 60, 100, 365}
	if got := Line(7*24*time.Hour+time.Hour, 1, ms); got != milestoneLines[7] {
		t.Errorf("day 7 should be the milestone line, got %q", got)
	}
	got := Line(16*24*time.Hour+3*time.Hour, 1, ms)
	if !strings.HasPrefix(got, "Sixteen days") {
		t.Errorf("got %q", got)
	}
	if Line(16*24*time.Hour+3*time.Hour, 1, ms) != Line(16*24*time.Hour+3*time.Hour+20*time.Minute, 1, ms) {
		t.Error("line should be stable within the hour")
	}
	if got := Line(10*time.Minute, 1, ms); !strings.Contains(strings.ToLower(got), "boot") && !strings.Contains(got, "minutes") && !strings.Contains(got, "Coffee") {
		t.Errorf("fresh boot line %q", got)
	}
	if got := Line(5*time.Hour, 1, ms); strings.Contains(got, "%") {
		t.Errorf("unreplaced placeholder in %q", got)
	}
}

func TestScreenFits(t *testing.T) {
	now := time.Date(2026, 9, 30, 14, 3, 0, 0, time.Local)
	boot := now.Add(-(16*24*time.Hour + 4*time.Hour + 51*time.Minute)).Unix()
	lines := Screen(now, boot, 23*24*time.Hour, &Funeral{Streak: 5 * 24 * time.Hour, EndedAt: now, NewRecord: true}, nil, 64)
	for _, l := range lines {
		if n := len([]rune(l)); n > 64 {
			t.Errorf("line too wide (%d): %q", n, l)
		}
	}
	if !strings.Contains(strings.Join(lines, "\n"), "16 days 4 hours 51 minutes") {
		t.Errorf("uptime missing:\n%s", strings.Join(lines, "\n"))
	}
}
