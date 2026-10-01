// Package gossip generates one line of speculation about a directory from
// its metadata alone: names, extensions, sizes and modification times. It
// never opens a file.
package gossip

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/words"
)

type Entry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// Scan reads up to max visible entries of dir.
func Scan(dir string, showHidden bool, max int) ([]Entry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	for len(out) < max {
		batch, err := f.ReadDir(256)
		for _, d := range batch {
			if len(out) >= max {
				break
			}
			if !showHidden && strings.HasPrefix(d.Name(), ".") {
				continue
			}
			info, ierr := d.Info()
			if ierr != nil {
				continue // vanished between readdir and lstat
			}
			out = append(out, Entry{
				Name:    d.Name(),
				IsDir:   d.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
		if err != nil {
			break // io.EOF or a real error; either way we have what we have
		}
	}
	return out, nil
}

// Candidate is one piece of gossip. Specific candidates (pairs, piles,
// suffixes) are preferred over generic ones (oldest, newest, largest).
type Candidate struct {
	Kind     string
	Text     string
	Specific bool
}

// Env carries the things Candidates needs from the outside world, so tests
// can supply fixed values.
type Env struct {
	Now          time.Time
	Rand         *rand.Rand
	AppInstalled func(token string) (appName string, ok bool)
}

const nameWidth = 40

func q(name string) string { return words.Truncate(name, nameWidth) }

func pick(r *rand.Rand, templates ...string) string { return templates[r.Intn(len(templates))] }

// Candidates builds every piece of gossip the directory supports.
func Candidates(entries []Entry, env Env) []Candidate {
	r := env.Rand
	var out []Candidate
	add := func(kind string, specific bool, text string) {
		out = append(out, Candidate{Kind: kind, Text: text, Specific: specific})
	}

	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	paired := map[string]bool{}

	// Related pairs: lockfiles and manifests.
	for _, p := range lockPairs(entries, byName) {
		paired[p[0]], paired[p[1]] = true, true
		a, b := q(p[0]), q(p[1])
		add("lock-pair", true, fmt.Sprintf(pick(r,
			"%s and %s are definitely related. Neither will comment.",
			"%[2]s follows %[1]s everywhere. It's getting a bit much.",
			"Everyone knows %[2]s only exists because of %[1]s.",
			"%s and %s are never seen apart. Make of that what you will.",
		), a, b))
	}

	// Source and what it became.
	for _, p := range buildPairs(entries, byName) {
		paired[p[0]], paired[p[1]] = true, true
		a, b := q(p[0]), q(p[1])
		add("build-pair", true, fmt.Sprintf(pick(r,
			"%[2]s used to be %[1]s. It doesn't like to talk about it.",
			"%s and %s look alike. Probably family.",
			"%[2]s is what %[1]s turned into. Some say it's not an improvement.",
		), a, b))
	}

	// An archive next to the folder that came out of it.
	for _, p := range archivePairs(entries, byName) {
		paired[p[0]], paired[p[1]] = true, true
		z, d := q(p[0]), q(p[1])
		add("archive-pair", true, fmt.Sprintf(pick(r,
			"%s and %s/ claim they've never met. They have the same name.",
			"%[2]s/ came out of %[1]s and never looked back.",
			"%[1]s is still here, even though %[2]s/ moved out.",
		), z, d))
	}

	// Same name, different extension, and nothing above explained it.
	for _, p := range stemPairs(entries, paired) {
		add("stem-pair", true, fmt.Sprintf(pick(r,
			"%s and %s share a name and refuse to explain it.",
			"%s and %s were seen leaving together.",
		), q(p[0]), q(p[1])))
	}

	// Version-suffix names.
	for _, e := range entries {
		if text := suffixGossip(e, r); text != "" {
			add("suffix", true, text)
		}
	}

	// Piles.
	piles := []struct {
		prefix string
		min    int
		texts  []string
	}{
		{"Screenshot ", 3, []string{
			"%d screenshots, and not one of them has been looked at twice.",
			"There are %d screenshots in here. Someone is documenting something.",
		}},
		{"Screen Recording ", 2, []string{
			"%d screen recordings. Evidence of something, surely.",
		}},
		{"Untitled", 2, []string{
			"%d files called Untitled. Nobody could think of anything.",
			"%d things named Untitled. Naming is hard. Apparently very hard.",
		}},
		{"IMG_", 5, []string{
			"%d photos called IMG_something. They all insist they're the good one.",
		}},
	}
	for _, p := range piles {
		n := 0
		for _, e := range entries {
			if strings.HasPrefix(e.Name, p.prefix) {
				n++
			}
		}
		if n >= p.min {
			add("pile", true, fmt.Sprintf(pick(r, p.texts...), n))
		}
	}

	// Installers that outstayed their welcome.
	for _, e := range entries {
		if e.IsDir || !strings.EqualFold(filepath.Ext(e.Name), ".dmg") {
			continue
		}
		token := dmgToken(e.Name)
		if token == "" || env.AppInstalled == nil {
			continue
		}
		if app, ok := env.AppInstalled(token); ok {
			add("dmg", true, fmt.Sprintf(pick(r,
				"%s already installed %s. It's just hanging around now.",
				"%s is still here. %s moved into Applications ages ago.",
			), q(e.Name), app))
		} else if age := env.Now.Sub(e.ModTime); age > 7*24*time.Hour {
			add("dmg", true, fmt.Sprintf(
				"%s has been waiting to be installed for %s. It's starting to take it personally.",
				q(e.Name), words.Duration(age)))
		}
	}

	// Empty files.
	for _, e := range entries {
		if !e.IsDir && e.Size == 0 {
			add("empty", true, fmt.Sprintf(pick(r,
				"%s is 0 bytes. It's not empty, it's minimalist.",
				"%s weighs nothing and contributes about the same.",
				"%s has nothing to say, and is saying it.",
			), q(e.Name)))
		}
	}

	// Extremes.
	if len(entries) >= 2 {
		sorted := append([]Entry(nil), entries...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].ModTime.Before(sorted[j].ModTime) })
		oldest, newest := sorted[0], sorted[len(sorted)-1]
		if env.Now.Sub(oldest.ModTime) > 30*24*time.Hour {
			add("oldest", false, fmt.Sprintf(pick(r,
				"%s hasn't changed since %s. It likes things the way they are.",
				"%s has been here since %s. It remembers when all this was empty.",
			), q(oldest.Name), oldest.ModTime.Format("2 Jan 2006")))
		}
		if ago := env.Now.Sub(newest.ModTime); ago >= 0 && ago < 2*24*time.Hour {
			add("newest", false, fmt.Sprintf(pick(r,
				"%s was touched %s ago. Everyone's talking about it.",
				"%s is the new arrival, as of %s ago. Nobody has said hello.",
			), q(newest.Name), words.Duration(ago)))
		}
	}
	var largest *Entry
	for i := range entries {
		e := &entries[i]
		if !e.IsDir && (largest == nil || e.Size > largest.Size) {
			largest = e
		}
	}
	if largest != nil && largest.Size >= 1<<20 {
		add("largest", false, fmt.Sprintf(pick(r,
			"%s takes up %s. It's not saying it's better than everyone, but it is bigger.",
			"%s is %s. Nobody asked it to take up that much room.",
		), q(largest.Name), words.Bytes(largest.Size)))
	}
	return out
}

// Choose picks one candidate, favouring specific gossip three times in four.
func Choose(cands []Candidate, r *rand.Rand) (Candidate, bool) {
	if len(cands) == 0 {
		return Candidate{}, false
	}
	var specific, generic []Candidate
	for _, c := range cands {
		if c.Specific {
			specific = append(specific, c)
		} else {
			generic = append(generic, c)
		}
	}
	pool := cands
	switch {
	case len(specific) > 0 && (len(generic) == 0 || r.Intn(4) > 0):
		pool = specific
	case len(generic) > 0:
		pool = generic
	}
	return pool[r.Intn(len(pool))], true
}

func splitExt(name string) (stem, ext string) {
	ext = filepath.Ext(name)
	if ext == name { // ".zshrc"
		return name, ""
	}
	return strings.TrimSuffix(name, ext), ext
}

var knownPairs = [][2]string{
	{"package.json", "package-lock.json"},
	{"package.json", "yarn.lock"},
	{"package.json", "pnpm-lock.yaml"},
	{"package.json", "bun.lockb"},
	{"go.mod", "go.sum"},
	{"Cargo.toml", "Cargo.lock"},
	{"pyproject.toml", "poetry.lock"},
	{"pyproject.toml", "uv.lock"},
	{"Gemfile", "Gemfile.lock"},
	{"Pipfile", "Pipfile.lock"},
	{"composer.json", "composer.lock"},
	{"Podfile", "Podfile.lock"},
	{"flake.nix", "flake.lock"},
}

func lockPairs(entries []Entry, byName map[string]Entry) [][2]string {
	var out [][2]string
	seen := map[string]bool{}
	addPair := func(a, b string) {
		if !seen[a+"\x00"+b] {
			seen[a+"\x00"+b] = true
			out = append(out, [2]string{a, b})
		}
	}
	for _, p := range knownPairs {
		if _, ok := byName[p[0]]; ok {
			if _, ok := byName[p[1]]; ok {
				addPair(p[0], p[1])
			}
		}
	}
	for _, e := range entries {
		stem, ext := splitExt(e.Name)
		if strings.HasSuffix(stem, "-lock") || ext == ".lock" {
			continue
		}
		if _, ok := byName[stem+"-lock"+ext]; ok && ext != "" {
			addPair(e.Name, stem+"-lock"+ext)
		}
		if _, ok := byName[e.Name+".lock"]; ok {
			addPair(e.Name, e.Name+".lock")
		}
	}
	return out
}

var buildExts = [][2]string{
	{".c", ".o"}, {".cpp", ".o"}, {".cc", ".o"}, {".m", ".o"}, {".swift", ".o"},
	{".java", ".class"}, {".py", ".pyc"}, {".ts", ".js"}, {".scss", ".css"},
	{".sass", ".css"}, {".less", ".css"}, {".md", ".html"}, {".md", ".pdf"},
	{".tex", ".pdf"}, {".docx", ".pdf"}, {".pages", ".pdf"}, {".psd", ".png"},
}

func buildPairs(entries []Entry, byName map[string]Entry) [][2]string {
	var out [][2]string
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		stem, ext := splitExt(e.Name)
		for _, be := range buildExts {
			if strings.EqualFold(ext, be[0]) {
				if o, ok := byName[stem+be[1]]; ok && !o.IsDir {
					out = append(out, [2]string{e.Name, o.Name})
				}
			}
		}
	}
	return out
}

var archiveExts = []string{".zip", ".tar.gz", ".tgz", ".tar", ".tar.xz", ".tar.bz2", ".7z", ".rar"}

func archivePairs(entries []Entry, byName map[string]Entry) [][2]string {
	var out [][2]string
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		lower := strings.ToLower(e.Name)
		for _, ae := range archiveExts {
			if strings.HasSuffix(lower, ae) && len(e.Name) > len(ae) {
				stem := e.Name[:len(e.Name)-len(ae)]
				if d, ok := byName[stem]; ok && d.IsDir {
					out = append(out, [2]string{e.Name, d.Name})
				}
				break
			}
		}
	}
	return out
}

func stemPairs(entries []Entry, paired map[string]bool) [][2]string {
	groups := map[string][]string{}
	var order []string
	for _, e := range entries {
		if e.IsDir || paired[e.Name] || strings.HasPrefix(e.Name, ".") {
			continue
		}
		stem, ext := splitExt(e.Name)
		if ext == "" {
			continue
		}
		if _, ok := groups[stem]; !ok {
			order = append(order, stem)
		}
		groups[stem] = append(groups[stem], e.Name)
	}
	var out [][2]string
	for _, stem := range order {
		if g := groups[stem]; len(g) == 2 {
			out = append(out, [2]string{g[0], g[1]})
		}
	}
	return out
}

var (
	finalVersion = regexp.MustCompile(`(?i)final[ _\-.]*v(?:ersion)?[ _\-.]*(\d+)`)
	suffixWord   = regexp.MustCompile(`(?i)(?:^|[ _\-.(])(final|copy|old|backup|bak|new|draft)(?:$|[ _\-.)\d])`)
	dupNumber    = regexp.MustCompile(`\(\d+\)$`)
)

func suffixGossip(e Entry, r *rand.Rand) string {
	stem, _ := splitExt(e.Name)
	if e.IsDir {
		stem = e.Name
	}
	n := q(e.Name)
	if m := finalVersion.FindStringSubmatch(stem); m != nil {
		var v int
		fmt.Sscanf(m[1], "%d", &v)
		if v >= 2 {
			if r.Intn(2) == 0 {
				return fmt.Sprintf("%s is the final version. The %s final version.", n, words.Ordinal(v))
			}
			before := "once"
			if v > 2 {
				before = words.Number(int64(v-1)) + " times"
			}
			return fmt.Sprintf("%s says it's final. It said that %s before.", n, before)
		}
	}
	if dupNumber.MatchString(stem) {
		return fmt.Sprintf("%s is a duplicate download. The original doesn't know.", n)
	}
	m := suffixWord.FindStringSubmatch(stem)
	if m == nil {
		return ""
	}
	switch strings.ToLower(m[1]) {
	case "final":
		return fmt.Sprintf(pick(r,
			"%s says it's final. Nobody believes it.",
			"%s is final, in the sense that nobody has changed it yet.",
		), n)
	case "copy":
		return fmt.Sprintf(pick(r,
			"%s is a copy and knows it. It's trying not to make it weird.",
			"%s insists it's the original.",
		), n)
	case "old":
		return fmt.Sprintf("%s is old, and everyone's too polite to delete it.", n)
	case "backup", "bak":
		return fmt.Sprintf("%s is a backup of something. It's no longer sure what.", n)
	case "new":
		return fmt.Sprintf("%s was new once.", n)
	case "draft":
		return fmt.Sprintf("%s is still a draft. It has been for some time.", n)
	}
	return ""
}

var tokenSplit = regexp.MustCompile(`[\s_\-.0-9]+`)

// dmgToken extracts the likely app name from an installer filename, e.g.
// "Firefox 130.0.dmg" -> "Firefox", "googlechrome.dmg" -> "googlechrome".
func dmgToken(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for _, f := range tokenSplit.Split(stem, -1) {
		if len(f) >= 3 {
			return f
		}
	}
	return ""
}

// InstalledApp looks in /Applications for an app whose name matches token,
// ignoring case and spaces.
func InstalledApp(token string) (string, bool) {
	ents, err := os.ReadDir("/Applications")
	if err != nil {
		return "", false
	}
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }
	t := norm(token)
	for _, e := range ents {
		name := strings.TrimSuffix(e.Name(), ".app")
		if name == e.Name() {
			continue
		}
		n := norm(name)
		if n == t || strings.HasPrefix(n, t) || strings.HasPrefix(t, n) && len(n) >= 4 {
			return name, true
		}
	}
	return "", false
}
