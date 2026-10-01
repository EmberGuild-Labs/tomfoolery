package eulogy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
	"github.com/EmberGuild-Labs/tomfoolery/internal/words"
)

// Proc is the metadata captured before the signal is sent.
type Proc struct {
	PID      int
	Name     string
	User     string
	Elapsed  time.Duration
	CPU      time.Duration
	RSSKB    int64
	Children int
}

// Snapshot is written to a temp file between --capture and --after.
type Snapshot struct {
	Signal string
	Procs  []Proc
}

// Capture reads metadata for pids with ps and pgrep.
func Capture(pids []int) []Proc {
	if len(pids) == 0 {
		return nil
	}
	list := make([]string, len(pids))
	for i, p := range pids {
		list[i] = strconv.Itoa(p)
	}
	// ps exits non-zero if any pid is missing, but still prints the rest.
	out, _ := exec.Command("/bin/ps", "-o", "pid=,etime=,time=,rss=,user=,comm=", "-p", strings.Join(list, ",")).Output()
	var procs []Proc
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		p, ok := parsePSLine(sc.Text())
		if !ok {
			continue
		}
		p.Children = countChildren(p.PID)
		procs = append(procs, p)
	}
	return procs
}

func parsePSLine(line string) (Proc, bool) {
	f := strings.Fields(line)
	if len(f) < 6 {
		return Proc{}, false
	}
	pid, err := strconv.Atoi(f[0])
	if err != nil {
		return Proc{}, false
	}
	rss, _ := strconv.ParseInt(f[3], 10, 64)
	// comm is the last column and may contain spaces; take everything after
	// the fifth field from the original line.
	rest := line
	for i := 0; i < 5; i++ {
		rest = strings.TrimLeft(rest, " \t")
		rest = rest[len(f[i]):]
	}
	comm := strings.TrimSpace(rest)
	return Proc{
		PID:     pid,
		Name:    filepath.Base(comm),
		User:    f[4],
		Elapsed: parseClock(f[1]),
		CPU:     parseClock(f[2]),
		RSSKB:   rss,
	}, true
}

// parseClock parses ps time formats: [[dd-]hh:]mm:ss[.cc].
func parseClock(s string) time.Duration {
	var days int64
	if i := strings.Index(s, "-"); i >= 0 {
		days, _ = strconv.ParseInt(s[:i], 10, 64)
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	var total float64
	for _, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		total = total*60 + v
	}
	return time.Duration(days)*24*time.Hour + time.Duration(total*float64(time.Second))
}

func countChildren(pid int) int {
	out, _ := exec.Command("/usr/bin/pgrep", "-P", strconv.Itoa(pid)).Output()
	return len(strings.Fields(string(out)))
}

// Pgrep runs pgrep with args and returns matching pids, excluding pids in skip.
func Pgrep(args []string, skip ...int) []int {
	out, _ := exec.Command("/usr/bin/pgrep", args...).Output()
	var pids []int
outer:
	for _, f := range strings.Fields(string(out)) {
		n, err := strconv.Atoi(f)
		if err != nil {
			continue
		}
		for _, s := range skip {
			if n == s {
				continue outer
			}
		}
		pids = append(pids, n)
	}
	return pids
}

// Save writes the snapshot to a temp file and returns its path.
func Save(s Snapshot) (string, error) {
	if err := os.MkdirAll(state.KillDir(), 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(state.KillDir(), "snap-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	return f.Name(), json.NewEncoder(f).Encode(s)
}

// Load reads and deletes a snapshot. It refuses paths outside the kill dir.
func Load(path string) (Snapshot, error) {
	var s Snapshot
	abs, err := filepath.Abs(path)
	if err != nil || filepath.Dir(abs) != state.KillDir() {
		return s, errors.New("snapshot path is not in the tomfoolery state directory")
	}
	b, err := os.ReadFile(abs)
	os.Remove(abs)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

// CleanStale removes snapshots left behind by interrupted runs.
func CleanStale() {
	ents, _ := os.ReadDir(state.KillDir())
	for _, e := range ents {
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
			os.Remove(filepath.Join(state.KillDir(), e.Name()))
		}
	}
}

// alive reports whether pid still exists and is not a zombie.
func alive(pid int, checkZombie bool) bool {
	err := syscall.Kill(pid, 0)
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	if checkZombie {
		out, _ := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		st := strings.TrimSpace(string(out))
		if st == "" || strings.HasPrefix(st, "Z") {
			return false
		}
	}
	return true
}

// WaitForDeaths polls until every process is gone or wait elapses, and
// returns which pids died.
func WaitForDeaths(procs []Proc, wait time.Duration) map[int]bool {
	dead := map[int]bool{}
	deadline := time.Now().Add(wait)
	for iter := 0; ; iter++ {
		remaining := 0
		for _, p := range procs {
			if dead[p.PID] {
				continue
			}
			if !alive(p.PID, iter%8 == 7) {
				dead[p.PID] = true
			} else {
				remaining++
			}
		}
		if remaining == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	// One last careful look at the survivors, in case they are zombies.
	for _, p := range procs {
		if !dead[p.PID] && !alive(p.PID, true) {
			dead[p.PID] = true
		}
	}
	return dead
}

var causes = map[string]string{
	"TERM": "Went peacefully",
	"INT":  "Interrupted mid-sentence",
	"HUP":  "Left when the terminal hung up",
	"KILL": "Taken without warning",
	"QUIT": "Quit, and dumped its thoughts on the way out",
	"ABRT": "Abandoned its own plans",
}

// Write renders the eulogy for a process that died.
func Write(p Proc, signal string, r *rand.Rand) string {
	var b strings.Builder
	fmt.Fprintf(&b, "In memory of %s (PID %d)\n", p.Name, p.PID)

	mem := words.Bytes(p.RSSKB * 1024)
	if p.CPU < time.Second {
		fmt.Fprintf(&b, "Born %s ago. Consumed almost no CPU and %s of memory.\n", words.Duration(p.Elapsed), mem)
	} else {
		fmt.Fprintf(&b, "Born %s ago. Consumed %s of CPU and %s of memory.\n",
			words.Duration(p.Elapsed), words.Duration(p.CPU), mem)
	}

	switch p.Children {
	case 0:
		if r.Intn(2) == 0 {
			b.WriteString("Leaves no child processes.\n")
		}
	case 1:
		b.WriteString("Survived by 1 child process.\n")
	default:
		fmt.Fprintf(&b, "Survived by %d child processes.\n", p.Children)
	}

	if p.Elapsed > 7*24*time.Hour {
		days := int64(p.Elapsed / (24 * time.Hour))
		fmt.Fprintf(&b, "%s for %s days. It had seen things. It had outlasted %s.\n",
			[]string{"A fixture of this machine", "A pillar of the process table"}[r.Intn(2)],
			words.Number(days),
			[]string{"several software updates", "countless browser tabs", "at least one Wi-Fi password"}[r.Intn(3)])
	}
	if p.CPU < time.Second && p.Elapsed > time.Hour {
		b.WriteString([]string{
			"Never did much. Never asked for much.\n",
			"It mostly waited. It was very good at waiting.\n",
		}[r.Intn(2)])
	}
	if p.User == "root" {
		b.WriteString("Ran as root, and never let anyone forget it.\n")
	}

	cause, ok := causes[signal]
	if !ok {
		cause = "Departed"
	}
	fmt.Fprintf(&b, "%s (SIG%s).\n", cause, signal)
	return b.String()
}

// Declined renders the note for a process that survived.
func Declined(p Proc, signal string) string {
	if signal == "KILL" {
		return fmt.Sprintf("%s (PID %d) has received the news. It is taking a moment.\n", p.Name, p.PID)
	}
	return fmt.Sprintf("%s (PID %d) has received the news and declined to die.\n", p.Name, p.PID)
}
