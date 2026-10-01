package eulogy

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestParsePSLine(t *testing.T) {
	p, ok := parsePSLine(" 4821 2-03:00:12   41:02.10 626688 alice   /Applications/Slack.app/Contents/Frameworks/Slack Helper.app/Contents/MacOS/Slack Helper")
	if !ok {
		t.Fatal("not parsed")
	}
	if p.PID != 4821 || p.Name != "Slack Helper" || p.User != "alice" || p.RSSKB != 626688 {
		t.Fatalf("got %+v", p)
	}
	if p.Elapsed != 51*time.Hour+12*time.Second {
		t.Errorf("elapsed %v", p.Elapsed)
	}
	if p.CPU.Truncate(time.Second) != 41*time.Minute+2*time.Second {
		t.Errorf("cpu %v", p.CPU)
	}
}

func TestWrite(t *testing.T) {
	p := Proc{PID: 4821, Name: "Slack Helper", Elapsed: 51 * time.Hour, CPU: 41 * time.Minute, RSSKB: 612 * 1024, Children: 6}
	got := Write(p, "TERM", rand.New(rand.NewSource(1)))
	for _, want := range []string{
		"In memory of Slack Helper (PID 4821)",
		"Born 2 days 3 hours ago. Consumed 41 minutes of CPU and 612 MB of memory.",
		"Survived by 6 child processes.",
		"Went peacefully (SIGTERM).",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if got := Write(Proc{Name: "x", Elapsed: 10 * 24 * time.Hour, CPU: 0}, "KILL", rand.New(rand.NewSource(1))); !strings.Contains(got, "Taken without warning (SIGKILL).") || !strings.Contains(got, "ten days") {
		t.Errorf("long-lived KILL eulogy:\n%s", got)
	}
}
