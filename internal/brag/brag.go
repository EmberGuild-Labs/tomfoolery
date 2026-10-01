// Package brag implements uptime-brag: the once-per-boot guard, the brag
// lines, the streak funeral and the redraw loop that runs in its window.
package brag

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
	"github.com/EmberGuild-Labs/tomfoolery/internal/words"
)

// BootTolerance absorbs small adjustments macOS makes to kern.boottime when
// it corrects the system clock.
const BootTolerance = 120

var bootSec = regexp.MustCompile(`sec\s*=\s*(\d+)`)

// BootTime returns the boot time in epoch seconds.
func BootTime() (int64, error) {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl kern.boottime: %w", err)
	}
	return ParseBootTime(string(out))
}

// ParseBootTime extracts sec from "{ sec = 1789000000, usec = 123456 } ...".
func ParseBootTime(s string) (int64, error) {
	m := bootSec.FindStringSubmatch(s)
	if m == nil {
		return 0, errors.New("could not parse kern.boottime")
	}
	return strconv.ParseInt(m[1], 10, 64)
}

// SameBoot reports whether two boot times refer to the same boot.
func SameBoot(a, b int64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= BootTolerance
}

// ShownThisBoot reports whether a window has already opened for this boot.
func ShownThisBoot(boot int64) bool {
	last, ok := state.ReadInt(state.LastShownBoot())
	return ok && SameBoot(last, boot)
}

// Funeral describes the streak that ended at the last reboot.
type Funeral struct {
	Streak    time.Duration
	EndedAt   time.Time
	NewRecord bool
}

// PrepareFuneral checks whether the last recorded uptime sample belongs to a
// previous boot. If it does, it stores a funeral for the next window, updates
// the record, and starts sampling the new boot. Safe to call repeatedly.
func PrepareFuneral(boot int64, now time.Time) error {
	seen, ok := state.ReadInts(state.LastSeen())
	if ok && len(seen) == 2 && !SameBoot(seen[0], boot) {
		streak := seen[1]
		rec, _ := state.ReadInt(state.Record())
		newRecord := streak > rec
		if newRecord {
			if err := state.WriteInts(state.Record(), streak); err != nil {
				return err
			}
		}
		flag := int64(0)
		if newRecord {
			flag = 1
		}
		if err := state.WriteInts(state.Funeral(), streak, seen[0]+streak, flag); err != nil {
			return err
		}
	}
	if !ok || len(seen) != 2 || !SameBoot(seen[0], boot) {
		return Sample(boot, now)
	}
	return nil
}

// Sample records the current uptime for a future funeral. The record itself
// is only raised at the funeral, once the streak is over.
func Sample(boot int64, now time.Time) error {
	up := now.Unix() - boot
	if up < 0 {
		up = 0
	}
	return state.WriteInts(state.LastSeen(), boot, up)
}

// TakeFuneral reads and removes the pending funeral, if any.
func TakeFuneral() (Funeral, bool) {
	v, ok := state.ReadInts(state.Funeral())
	state.Remove(state.Funeral())
	if !ok || len(v) != 3 {
		return Funeral{}, false
	}
	return Funeral{
		Streak:    time.Duration(v[0]) * time.Second,
		EndedAt:   time.Unix(v[1], 0),
		NewRecord: v[2] == 1,
	}, true
}

// Record returns the longest recorded streak.
func Record() time.Duration {
	v, _ := state.ReadInt(state.Record())
	return time.Duration(v) * time.Second
}

const day = 24 * time.Hour

func daysPhrase(d time.Duration) string {
	n := int64(d / day)
	if n == 1 {
		return "One day"
	}
	return words.Capitalize(words.Number(n)) + " days"
}

var milestoneLines = map[int]string{
	1:   "One full day. The first of many, it says.",
	3:   "Three days. It's getting comfortable.",
	7:   "One week. It wants you to know it noticed.",
	14:  "Two weeks. It has begun to strut.",
	30:  "Thirty days. A whole month. It is insufferable now.",
	60:  "Sixty days. It has started a newsletter.",
	100: "One hundred days. It would like a parade.",
	365: "One year. It demands a plaque.",
}

type tier struct {
	upTo  time.Duration
	lines []string
}

// %D is replaced with "Sixteen days", %H with "five hours".
var tiers = []tier{
	{time.Hour, []string{
		"Fresh boot. Nothing to brag about yet. Give it time.",
		"Just woke up. Coffee first, then bragging.",
		"Uptime measured in minutes. Humble beginnings.",
	}},
	{day, []string{
		"%H and counting. Modest, but honest.",
		"Not even a day yet. The brag is pending.",
		"Still on day one. Everyone starts somewhere.",
	}},
	{3 * day, []string{
		"%D. Mildly pleased with itself.",
		"%D without a restart. Quietly proud.",
		"%D up. Nothing crashed. Noted.",
	}},
	{14 * day, []string{
		"%D. Your software updates have not been this patient.",
		"%D without a reboot. Smug, but earned.",
		"%D. The restart button is starting to feel left out.",
	}},
	{30 * day, []string{
		"%D. It has stopped asking when the next restart is.",
		"%D. Other Macs are starting to talk.",
		"%D. It considers \"Restart to Update\" a personal insult.",
	}},
	{100 * day, []string{
		"%D. Openly insufferable about it.",
		"%D. It has started referring to itself as \"the server\".",
		"%D. Please address it as Uptime from now on.",
	}},
	{365 * day, []string{
		"%D. It is no longer a computer, it is a landmark.",
		"%D. Software updates have stopped asking.",
		"%D. Geologists have expressed interest.",
	}},
	{1 << 62, []string{
		"%D. This Mac demands a plaque.",
		"%D. A year or more. Somebody call a notary.",
	}},
}

// Line returns the brag for an uptime. Milestone days get their special line
// for the whole of that day. Other lines rotate hourly, seeded by the boot,
// so the window doesn't flicker between brags every redraw.
func Line(up time.Duration, boot int64, milestones []int) string {
	days := int(up / day)
	for _, m := range milestones {
		if days == m {
			if l, ok := milestoneLines[m]; ok {
				return l
			}
			return daysPhrase(up) + ". A milestone. It will be mentioning this."
		}
	}
	h := fnv.New64a()
	fmt.Fprintf(h, "%d/%d", boot, int64(up/time.Hour))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	for _, t := range tiers {
		if up < t.upTo {
			l := t.lines[r.Intn(len(t.lines))]
			l = strings.ReplaceAll(l, "%D", daysPhrase(up))
			hours := int64(up / time.Hour)
			hp := words.Capitalize(words.Number(hours)) + " hours"
			if hours == 1 {
				hp = "One hour"
			}
			return strings.ReplaceAll(l, "%H", hp)
		}
	}
	return ""
}

// Screen renders the window contents as lines.
func Screen(now time.Time, boot int64, record time.Duration, fun *Funeral, milestones []int, cols int) []string {
	up := now.Sub(time.Unix(boot, 0))
	if up < 0 {
		up = 0
	}
	best := record
	if up > best {
		best = up
	}
	width := cols - 4
	lines := []string{
		"",
		"  UPTIME BRAG",
		"",
		"  Up since   " + time.Unix(boot, 0).Format("Mon 2 Jan 15:04"),
		"  Uptime     " + words.LongDuration(up),
		"  Best       " + words.LongDuration(best),
		"",
	}
	for _, l := range words.Wrap(Line(up, boot, milestones), width) {
		lines = append(lines, "  "+l)
	}
	if fun != nil {
		lines = append(lines, "")
		text := fmt.Sprintf("In memory of the previous streak: %s, ended around %s. It went the way of all uptime.",
			words.LongDuration(fun.Streak), fun.EndedAt.Format("Mon 2 Jan 15:04"))
		if fun.NewRecord {
			text += " It set a new record. Gone, but on the leaderboard."
		}
		for _, l := range words.Wrap(text, width) {
			lines = append(lines, "  "+l)
		}
	}
	return lines
}
