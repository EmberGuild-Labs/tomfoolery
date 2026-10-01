package config

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

func TestDefaultFileMatchesDefaults(t *testing.T) {
	cfg, err := Parse(DefaultFileText)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("DefaultFileText disagrees with Default():\n%+v\n%+v", cfg, Default())
	}
}

func TestParse(t *testing.T) {
	cfg, err := Parse(`
[ls_gossip]
frequency = 20 # less gossip
style = 'plain'

[kill_eulogy]
wrap = [
  "kill",   # always
  "pkill",
]

[uptime_brag]
terminal = "iterm"
focus = true
milestones_days = [10, 20]
`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LS.Frequency != 20 || cfg.LS.Style != "plain" || cfg.Brag.Terminal != "iTerm" || !cfg.Brag.Focus {
		t.Errorf("%+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Kill.Wrap, []string{"kill", "pkill"}) || !reflect.DeepEqual(cfg.Brag.MilestonesDays, []int{10, 20}) {
		t.Errorf("%+v", cfg)
	}
	if cfg.LS.CooldownSeconds != 20 {
		t.Error("unset keys keep defaults")
	}
}

func TestParseErrors(t *testing.T) {
	for _, text := range []string{
		"[ls_gossip]\nfrequency = 150",
		"[ls_gossip]\nfrequency = \"lots\"",
		"[kill_eulogy]\nwrap = [\"rm\"]",
		"[ls_gossip\n",
		"just words",
	} {
		cfg, err := Parse(text)
		if err == nil {
			t.Errorf("expected error for %q", text)
		}
		if cfg.LS.Frequency != 35 && !strings.Contains(text, "frequency") {
			t.Errorf("bad file should still yield defaults")
		}
	}
}

func TestSetLSFrequency(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SetLSFrequency(12); err != nil { // no file yet: writes the default file
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil || cfg.LS.Frequency != 12 {
		t.Fatalf("%v %v", cfg.LS.Frequency, err)
	}
	b, _ := os.ReadFile(state.ConfigFile())
	if !strings.Contains(string(b), "frequency = 12              # percent chance") {
		t.Errorf("comment not preserved:\n%s", b)
	}
	os.WriteFile(state.ConfigFile(), []byte("[uptime_brag]\nfocus = true\n"), 0o644)
	SetLSFrequency(50)
	cfg, _ = Load()
	if cfg.LS.Frequency != 50 || !cfg.Brag.Focus {
		t.Errorf("%+v", cfg)
	}
}
