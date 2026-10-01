package gossip

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

func ent(name string, size int64, age time.Duration) Entry {
	return Entry{Name: name, Size: size, ModTime: now.Add(-age)}
}

func kinds(cands []Candidate) map[string][]string {
	m := map[string][]string{}
	for _, c := range cands {
		m[c.Kind] = append(m[c.Kind], c.Text)
	}
	return m
}

func TestCandidates(t *testing.T) {
	entries := []Entry{
		ent("package.json", 900, time.Hour),
		ent("package-lock.json", 90000, time.Hour),
		ent("main.c", 100, 24*time.Hour),
		ent("main.o", 100, 24*time.Hour),
		ent("notes_final_v3.txt", 10, 40*24*time.Hour),
		ent("Screenshot 2026-09-12 at 14.03.11.png", 1, 2*time.Hour),
		ent("Screenshot 2026-09-13 at 14.03.11.png", 1, 3*time.Hour),
		ent("Screenshot 2026-09-14 at 14.03.11.png", 1, 4*time.Hour),
		ent("empty.txt", 0, time.Minute),
		ent("Firefox 130.0.dmg", 200<<20, 30*24*time.Hour),
		ent("site.zip", 5000, 60*24*time.Hour),
		{Name: "site", IsDir: true, ModTime: now.Add(-60 * 24 * time.Hour)},
		ent("report (1).pdf", 5000, 3*time.Hour),
	}
	env := Env{Now: now, Rand: rand.New(rand.NewSource(1)), AppInstalled: func(tok string) (string, bool) {
		if tok == "Firefox" {
			return "Firefox", true
		}
		return "", false
	}}
	k := kinds(Candidates(entries, env))
	for _, want := range []string{"lock-pair", "build-pair", "archive-pair", "suffix", "pile", "dmg", "empty", "oldest", "newest", "largest"} {
		if len(k[want]) == 0 {
			t.Errorf("no %s gossip; got %v", want, k)
		}
	}
	if !strings.Contains(strings.Join(k["lock-pair"], ""), "package-lock.json") {
		t.Errorf("lock pair text: %v", k["lock-pair"])
	}
	if !strings.Contains(strings.Join(k["pile"], ""), "3 screenshots") {
		t.Errorf("pile text: %v", k["pile"])
	}
	if !strings.Contains(strings.Join(k["suffix"], " "), "notes_final_v3.txt") {
		t.Errorf("suffix text: %v", k["suffix"])
	}
	for _, c := range Candidates(entries, env) {
		if strings.Contains(c.Text, "%!") || strings.Contains(c.Text, "%s") {
			t.Errorf("bad format: %q", c.Text)
		}
	}
}

func TestNoFalseGossip(t *testing.T) {
	entries := []Entry{ent("alpha", 10, time.Hour), ent("beta", 10, time.Hour), ent("gamma", 10, time.Hour), ent("oldness.md", 10, time.Hour)}
	k := kinds(Candidates(entries, Env{Now: now, Rand: rand.New(rand.NewSource(1))}))
	for _, bad := range []string{"lock-pair", "build-pair", "suffix", "pile", "empty", "largest", "oldest"} {
		if len(k[bad]) > 0 {
			t.Errorf("unexpected %s: %v", bad, k[bad])
		}
	}
}

func TestChoosePrefersSpecific(t *testing.T) {
	cands := []Candidate{{Kind: "a", Specific: true}, {Kind: "b"}}
	r := rand.New(rand.NewSource(1))
	spec := 0
	for i := 0; i < 1000; i++ {
		if c, _ := Choose(cands, r); c.Specific {
			spec++
		}
	}
	if spec < 650 || spec > 850 {
		t.Errorf("specific chosen %d/1000, want about 750", spec)
	}
	if _, ok := Choose(nil, r); ok {
		t.Error("empty choose should fail")
	}
}

func TestParseLsArgs(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "file"), nil, 0o644)
	os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "link"))
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)

	ok := func(args ...string) Target {
		t.Helper()
		tg, ok := ParseLsArgs(args)
		if !ok {
			t.Fatalf("ParseLsArgs(%q) should be ok", args)
		}
		return tg
	}
	bad := func(args ...string) {
		t.Helper()
		if _, ok := ParseLsArgs(args); ok {
			t.Errorf("ParseLsArgs(%q) should be refused", args)
		}
	}
	if tg := ok(); tg.Dir != "." || tg.ShowHidden {
		t.Errorf("%+v", tg)
	}
	if tg := ok("-la"); !tg.ShowHidden || !tg.ShowDotNames {
		t.Errorf("%+v", tg)
	}
	if tg := ok("-A", "sub"); !tg.ShowHidden || tg.ShowDotNames || tg.Dir != "sub" {
		t.Errorf("%+v", tg)
	}
	ok("-G", "--color=auto", "sub")
	ok("link")
	ok("-l", "link/")
	ok("-lH", "link")
	ok("--", "sub")
	bad("-l", "link")
	bad("file")
	bad("sub", ".")
	bad("-d", "sub")
	bad("-R")
	bad("missing")
	bad("sub", "-l")
	bad("--bogus")
}
