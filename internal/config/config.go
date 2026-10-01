// Package config loads ~/.config/tomfoolery/config.toml. The file is
// optional; every setting has a default.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

type LSGossip struct {
	Frequency         int
	CooldownSeconds   int
	MinEntries        int
	MaxEntriesScanned int
	Style             string
}

type KillEulogy struct {
	Wrap        []string
	DeathWaitMS int
}

type UptimeBrag struct {
	Terminal                string
	Focus                   bool
	WindowCols              int
	WindowRows              int
	RedrawSeconds           int
	LastSeenIntervalMinutes int
	MilestonesDays          []int
}

type Config struct {
	LS   LSGossip
	Kill KillEulogy
	Brag UptimeBrag
}

func Default() Config {
	return Config{
		LS: LSGossip{
			Frequency:         35,
			CooldownSeconds:   20,
			MinEntries:        4,
			MaxEntriesScanned: 2000,
			Style:             "dim",
		},
		Kill: KillEulogy{
			Wrap:        []string{"kill"},
			DeathWaitMS: 1000,
		},
		Brag: UptimeBrag{
			Terminal:                "Terminal",
			Focus:                   false,
			WindowCols:              64,
			WindowRows:              10,
			RedrawSeconds:           60,
			LastSeenIntervalMinutes: 5,
			MilestonesDays:          []int{1, 3, 7, 14, 30, 60, 100, 365},
		},
	}
}

// DefaultFileText is written by `tomfoolery install` when no config exists.
const DefaultFileText = `# tomfoolery configuration. Every setting is optional; these are the defaults.

[ls_gossip]
frequency = 35              # percent chance per eligible run (0-100)
cooldown_seconds = 20
min_entries = 4
max_entries_scanned = 2000
style = "dim"               # "dim" or "plain"

[kill_eulogy]
wrap = ["kill"]             # add "pkill" and/or "killall" (open a new shell after changing)
death_wait_ms = 1000        # max wait for the process to disappear

[uptime_brag]
terminal = "Terminal"       # "Terminal" or "iTerm"
focus = false               # true = bring window to front when opened
window_cols = 64
window_rows = 10
redraw_seconds = 60
last_seen_interval_minutes = 5
milestones_days = [1, 3, 7, 14, 30, 60, 100, 365]
`

// Load reads the config file if present. A missing file yields defaults. A
// malformed file yields defaults plus an error describing the problem, so
// callers can keep working and still report it.
func Load() (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(state.ConfigFile())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return Parse(string(b))
}

// Parse applies the TOML text on top of the defaults.
func Parse(text string) (Config, error) {
	cfg := Default()
	doc, err := parseTOML(text)
	if err != nil {
		return cfg, fmt.Errorf("config.toml: %w", err)
	}
	var errs []string
	t := doc["ls_gossip"]
	getInt(t, "frequency", &cfg.LS.Frequency, 0, 100, &errs)
	getInt(t, "cooldown_seconds", &cfg.LS.CooldownSeconds, 0, 1<<30, &errs)
	getInt(t, "min_entries", &cfg.LS.MinEntries, 0, 1<<30, &errs)
	getInt(t, "max_entries_scanned", &cfg.LS.MaxEntriesScanned, 1, 1<<30, &errs)
	getEnum(t, "style", &cfg.LS.Style, []string{"dim", "plain"}, &errs)

	t = doc["kill_eulogy"]
	if v, ok := t["wrap"]; ok {
		arr, isArr := v.([]any)
		var wrap []string
		for _, item := range arr {
			s, _ := item.(string)
			if s != "kill" && s != "pkill" && s != "killall" {
				isArr = false
				break
			}
			wrap = append(wrap, s)
		}
		if isArr {
			cfg.Kill.Wrap = wrap
		} else {
			errs = append(errs, `kill_eulogy.wrap must be a list of "kill", "pkill", "killall"`)
		}
	}
	getInt(t, "death_wait_ms", &cfg.Kill.DeathWaitMS, 0, 10000, &errs)

	t = doc["uptime_brag"]
	getEnum(t, "terminal", &cfg.Brag.Terminal, []string{"Terminal", "iTerm"}, &errs)
	if v, ok := t["focus"]; ok {
		if b, isBool := v.(bool); isBool {
			cfg.Brag.Focus = b
		} else {
			errs = append(errs, "uptime_brag.focus must be true or false")
		}
	}
	getInt(t, "window_cols", &cfg.Brag.WindowCols, 30, 300, &errs)
	getInt(t, "window_rows", &cfg.Brag.WindowRows, 6, 100, &errs)
	getInt(t, "redraw_seconds", &cfg.Brag.RedrawSeconds, 5, 3600, &errs)
	getInt(t, "last_seen_interval_minutes", &cfg.Brag.LastSeenIntervalMinutes, 1, 1440, &errs)
	if v, ok := t["milestones_days"]; ok {
		arr, isArr := v.([]any)
		var days []int
		for _, item := range arr {
			n, isInt := item.(int64)
			if !isInt || n < 1 {
				isArr = false
				break
			}
			days = append(days, int(n))
		}
		if isArr {
			cfg.Brag.MilestonesDays = days
		} else {
			errs = append(errs, "uptime_brag.milestones_days must be a list of positive integers")
		}
	}
	if len(errs) > 0 {
		return cfg, fmt.Errorf("config.toml: %s", strings.Join(errs, "; "))
	}
	return cfg, nil
}

func getInt(t map[string]any, key string, dst *int, lo, hi int, errs *[]string) {
	v, ok := t[key]
	if !ok {
		return
	}
	n, isInt := v.(int64)
	if !isInt || n < int64(lo) || n > int64(hi) {
		*errs = append(*errs, fmt.Sprintf("%s must be an integer from %d to %d", key, lo, hi))
		return
	}
	*dst = int(n)
}

func getEnum(t map[string]any, key string, dst *string, allowed []string, errs *[]string) {
	v, ok := t[key]
	if !ok {
		return
	}
	s, _ := v.(string)
	for _, a := range allowed {
		if strings.EqualFold(s, a) {
			*dst = a
			return
		}
	}
	*errs = append(*errs, fmt.Sprintf("%s must be one of %q", key, allowed))
}

var freqLine = regexp.MustCompile(`^(\s*frequency\s*=\s*)\d+(.*)$`)

// SetLSFrequency stores a new ls-gossip frequency in config.toml, editing
// the existing line in place (keeping comments) or adding one.
func SetLSFrequency(n int) error {
	path := state.ConfigFile()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		b, err = []byte(DefaultFileText), nil
	}
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	section, sectionAt, done := "", -1, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[] ")
			if section == "ls_gossip" {
				sectionAt = i
			}
			continue
		}
		if section == "ls_gossip" && freqLine.MatchString(line) {
			lines[i] = freqLine.ReplaceAllString(line, "${1}"+strconv.Itoa(n)+"${2}")
			done = true
			break
		}
	}
	if !done {
		entry := "frequency = " + strconv.Itoa(n)
		if sectionAt >= 0 {
			lines = append(lines[:sectionAt+1], append([]string{entry}, lines[sectionAt+1:]...)...)
		} else {
			lines = append(lines, "", "[ls_gossip]", entry)
		}
	}
	return state.Write(path, strings.Join(lines, "\n"), 0o644)
}
