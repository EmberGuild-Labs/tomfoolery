package gossip

import (
	"os"
	"strings"
)

// Target describes the single directory an `ls` invocation lists, if any.
type Target struct {
	Dir          string
	ShowHidden   bool // -a, -A or -f: dotfiles appear in the listing
	ShowDotNames bool // -a or -f: "." and ".." also appear (never gossiped about)
}

// ParseLsArgs works out which directory macOS `ls` listed. It returns ok=false
// whenever that is not exactly one directory, or when the flags make the
// answer uncertain. BSD ls stops reading flags at the first operand.
func ParseLsArgs(args []string) (Target, bool) {
	t := Target{}
	long, classify, recursive, dirsAsFiles, followLinks := false, false, false, false, false
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			break
		}
		if strings.HasPrefix(a, "--") {
			// macOS ls only knows --color[=when]; anything else makes ls fail.
			if a == "--color" || strings.HasPrefix(a, "--color=") {
				continue
			}
			return t, false
		}
		for _, c := range a[1:] {
			switch c {
			case 'a':
				t.ShowHidden, t.ShowDotNames = true, true
			case 'f':
				t.ShowHidden, t.ShowDotNames = true, true
			case 'A':
				t.ShowHidden = true
			case 'l', 'n', 'o', 'g':
				long = true
			case 'F':
				classify = true
			case 'R':
				recursive = true
			case 'd':
				dirsAsFiles = true
			case 'H', 'L':
				followLinks = true
			case 'D':
				// -D takes an argument (a strftime format); too clever for us.
				return t, false
			}
		}
	}
	if recursive || dirsAsFiles {
		return t, false
	}
	operands := args[i:]
	switch len(operands) {
	case 0:
		t.Dir = "."
	case 1:
		t.Dir = operands[0]
	default:
		return t, false
	}
	info, err := os.Lstat(t.Dir)
	if err != nil {
		return t, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// A symlink operand is listed as the link itself under -l or -F,
		// unless -H/-L or a trailing slash say to follow it.
		if (long || classify) && !followLinks && !strings.HasSuffix(t.Dir, "/") {
			return t, false
		}
		info, err = os.Stat(t.Dir)
		if err != nil {
			return t, false
		}
	}
	if !info.IsDir() {
		return t, false
	}
	return t, true
}
