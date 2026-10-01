// Package eulogy captures process metadata before a kill and writes a short
// eulogy afterwards for every process that actually died.
package eulogy

import (
	"strconv"
	"strings"
)

// Darwin signal numbers.
var signalNumbers = map[string]int{
	"HUP": 1, "INT": 2, "QUIT": 3, "ILL": 4, "TRAP": 5, "ABRT": 6, "IOT": 6, "EMT": 7,
	"FPE": 8, "KILL": 9, "BUS": 10, "SEGV": 11, "SYS": 12, "PIPE": 13, "ALRM": 14,
	"TERM": 15, "URG": 16, "STOP": 17, "TSTP": 18, "CONT": 19, "CHLD": 20, "TTIN": 21,
	"TTOU": 22, "IO": 23, "POLL": 23, "XCPU": 24, "XFSZ": 25, "VTALRM": 26, "PROF": 27,
	"WINCH": 28, "INFO": 29, "USR1": 30, "USR2": 31,
}

// terminating is the set of signals that get a eulogy. Everything else
// (USR1, STOP, CONT, WINCH, ...) passes straight through.
var terminating = map[string]bool{
	"HUP": true, "INT": true, "QUIT": true, "ABRT": true, "KILL": true, "TERM": true,
}

// SignalName canonicalises "sigkill", "KILL", "SIGKILL" or "9" to "KILL".
// It returns "" for anything it does not recognise.
func SignalName(s string) string {
	if s == "" {
		return ""
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n == 6 {
			return "ABRT"
		}
		if n == 23 {
			return "IO"
		}
		for name, num := range signalNumbers {
			if num == n && name != "IOT" && name != "POLL" {
				return name
			}
		}
		return ""
	}
	up := strings.ToUpper(s)
	up = strings.TrimPrefix(up, "SIG")
	if _, ok := signalNumbers[up]; ok {
		switch up {
		case "IOT":
			return "ABRT"
		case "POLL":
			return "IO"
		}
		return up
	}
	return ""
}

// Plan is what the wrapper intends to eulogize.
type Plan struct {
	Signal string
	PIDs   []int
}

// ParseKill interprets arguments to the kill builtin. ok=false means "pass
// straight through without a eulogy": -l, -0, job specs, process groups,
// non-terminating signals, and anything not understood with certainty.
func ParseKill(args []string) (Plan, bool) {
	p := Plan{Signal: "TERM"}
	if len(args) == 0 {
		return p, false
	}
	i := 0
	switch a := args[0]; {
	case a == "-s" || a == "-n":
		if len(args) < 2 {
			return p, false
		}
		if a == "-n" {
			if _, err := strconv.Atoi(args[1]); err != nil {
				return p, false
			}
		}
		p.Signal = SignalName(args[1])
		i = 2
	case a == "--":
		// handled below
	case strings.HasPrefix(a, "-") && len(a) > 1:
		p.Signal = SignalName(a[1:])
		i = 1
	}
	if p.Signal == "" || !terminating[p.Signal] {
		return p, false
	}
	if i < len(args) && args[i] == "--" {
		i++
	}
	pids, ok := parsePIDs(args[i:])
	if !ok {
		return p, false
	}
	p.PIDs = pids
	return p, true
}

func parsePIDs(args []string) ([]int, bool) {
	if len(args) == 0 {
		return nil, false
	}
	var pids []int
	for _, a := range args {
		if a == "" || a[0] < '0' || a[0] > '9' {
			return nil, false // %job, -pgid, names
		}
		n, err := strconv.Atoi(a)
		if err != nil || n <= 1 { // 0 is "my process group"; 1 is launchd
			return nil, false
		}
		pids = append(pids, n)
	}
	return pids, true
}

// ParsePkill splits pkill arguments into a signal and the arguments pgrep
// needs to find the same processes. ok=false means pass through.
func ParsePkill(args []string) (sig string, pgrepArgs []string, ok bool) {
	sig = "TERM"
	rest := args
	if len(rest) > 0 && strings.HasPrefix(rest[0], "-") && len(rest[0]) > 1 {
		cand := rest[0][1:]
		// pkill takes -SIGNAL first. Only trust digits or upper-case names,
		// because lower-case letters are pgrep flags (-f, -l, -n, ...).
		if _, err := strconv.Atoi(cand); err == nil || cand == strings.ToUpper(cand) {
			if s := SignalName(cand); s != "" {
				sig = s
				rest = rest[1:]
			}
		}
	}
	if !terminating[sig] {
		return sig, nil, false
	}
	for _, a := range rest {
		// -I asks for confirmation and -l changes the output format.
		if strings.HasPrefix(a, "-") && (strings.ContainsAny(a, "Il") && !strings.HasPrefix(a, "--")) {
			return sig, nil, false
		}
	}
	if len(rest) == 0 {
		return sig, nil, false
	}
	return sig, rest, true
}

// ParseKillall accepts only `killall [-SIGNAL] name...`; every other flag
// passes through, since killall's options differ too much from pgrep's.
func ParseKillall(args []string) (sig string, names []string, ok bool) {
	sig = "TERM"
	rest := args
	if len(rest) > 0 && strings.HasPrefix(rest[0], "-") && len(rest[0]) > 1 {
		s := SignalName(rest[0][1:])
		if s == "" {
			return sig, nil, false
		}
		sig = s
		rest = rest[1:]
	}
	if !terminating[sig] || len(rest) == 0 {
		return sig, nil, false
	}
	for _, a := range rest {
		if strings.HasPrefix(a, "-") {
			return sig, nil, false
		}
	}
	return sig, rest, true
}
