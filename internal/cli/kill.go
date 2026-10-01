package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/config"
	"github.com/EmberGuild-Labs/tomfoolery/internal/eulogy"
	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

// At most this many full eulogies per kill; the rest are summarized.
const maxEulogies = 5

func killWithEulogy(args []string) int {
	if len(args) == 0 {
		args = []string{"status"}
	}
	switch args[0] {
	case "--capture":
		if len(args) < 2 {
			return 0
		}
		rest := args[2:]
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		capture(args[1], rest)
		return 0
	case "--after":
		if len(args) == 2 {
			after(args[1])
		}
		return 0
	case "on", "off":
		if err := setMarker(state.KillOff(), args[0] == "off"); err != nil {
			return fail("kill-with-eulogy", err)
		}
		fmt.Fprintf(stdout, "kill-with-eulogy is %s.\n", args[0])
		return 0
	case "status":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(stderr, "warning:", err)
		}
		fmt.Fprintf(stdout, "kill-with-eulogy is %s. Wrapping: %s.\n",
			onOff(!state.Exists(state.KillOff())), strings.Join(cfg.Kill.Wrap, ", "))
		return 0
	}
	fmt.Fprintf(stderr, "kill-with-eulogy: unknown command %q (try on, off, status)\n", args[0])
	return 2
}

// capture prints the path of a snapshot file, or nothing if the call
// should pass straight through.
func capture(cmd string, args []string) {
	defer func() { recover() }()
	eulogy.CleanStale()
	var sig string
	var pids []int
	// Never eulogize ourselves or the $(...) subshell that runs us.
	skip := []int{os.Getpid(), os.Getppid()}
	switch cmd {
	case "kill":
		plan, ok := eulogy.ParseKill(args)
		if !ok {
			return
		}
		sig, pids = plan.Signal, plan.PIDs
	case "pkill":
		s, pgrepArgs, ok := eulogy.ParsePkill(args)
		if !ok {
			return
		}
		sig, pids = s, eulogy.Pgrep(pgrepArgs, skip...)
	case "killall":
		s, names, ok := eulogy.ParseKillall(args)
		if !ok {
			return
		}
		sig = s
		for _, n := range names {
			pgrepArgs := []string{"-x", n}
			if os.Getuid() != 0 { // killall only targets your own processes
				pgrepArgs = append([]string{"-U", strconv.Itoa(os.Getuid())}, pgrepArgs...)
			}
			pids = append(pids, eulogy.Pgrep(pgrepArgs, skip...)...)
		}
	default:
		return
	}
	procs := eulogy.Capture(pids)
	if len(procs) == 0 {
		return
	}
	path, err := eulogy.Save(eulogy.Snapshot{Signal: sig, Procs: procs})
	if err != nil {
		return
	}
	fmt.Fprint(stdout, path)
}

func after(path string) {
	defer func() { recover() }()
	snap, err := eulogy.Load(path)
	if err != nil || len(snap.Procs) == 0 {
		return
	}
	cfg, _ := config.Load()
	dead := eulogy.WaitForDeaths(snap.Procs, time.Duration(cfg.Kill.DeathWaitMS)*time.Millisecond)
	r := newRand()
	var blocks []string
	extra := 0
	for _, p := range snap.Procs {
		switch {
		case !dead[p.PID]:
			blocks = append(blocks, eulogy.Declined(p, snap.Signal))
		case len(blocks) < maxEulogies:
			blocks = append(blocks, eulogy.Write(p, snap.Signal, r))
		default:
			extra++
		}
	}
	if extra > 0 {
		blocks = append(blocks, fmt.Sprintf("...and %s more, who will be remembered collectively.\n", strconv.Itoa(extra)))
	}
	fmt.Fprint(stdout, strings.Join(blocks, "\n"))
}
