package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/config"
	"github.com/EmberGuild-Labs/tomfoolery/internal/gossip"
	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

func lsGossip(args []string) int {
	if len(args) == 0 {
		args = []string{"status"}
	}
	switch args[0] {
	case "--maybe":
		rest := args[1:]
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		maybeGossip(rest)
		return 0 // never let gossip affect ls
	case "on", "off":
		if err := setMarker(state.LSOff(), args[0] == "off"); err != nil {
			return fail("ls-gossip", err)
		}
		fmt.Fprintf(stdout, "ls-gossip is %s.\n", args[0])
		return 0
	case "status":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(stderr, "warning:", err)
		}
		fmt.Fprintf(stdout, "ls-gossip is %s. Frequency %d%%, cooldown %ds, minimum %d entries.\n",
			onOff(!state.Exists(state.LSOff())), cfg.LS.Frequency, cfg.LS.CooldownSeconds, cfg.LS.MinEntries)
		syncFreqCache(cfg)
		return 0
	case "now":
		dir := "."
		if len(args) > 1 {
			dir = args[1]
		}
		return gossipNow(dir)
	case "freq":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: ls-gossip freq <0-100>")
			return 2
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 || n > 100 {
			fmt.Fprintln(stderr, "ls-gossip: frequency must be a whole number from 0 to 100")
			return 2
		}
		if err := config.SetLSFrequency(n); err != nil {
			return fail("ls-gossip", err)
		}
		if err := state.WriteInts(state.LSFreq(), int64(n)); err != nil {
			return fail("ls-gossip", err)
		}
		fmt.Fprintf(stdout, "ls-gossip will now gossip on %d%% of eligible runs.\n", n)
		return 0
	}
	fmt.Fprintf(stderr, "ls-gossip: unknown command %q (try on, off, status, now, freq)\n", args[0])
	return 2
}

// syncFreqCache keeps the plain-number file the shell hook reads in step
// with config.toml, so the dice roll needs no process spawn.
func syncFreqCache(cfg config.Config) {
	if v, ok := state.ReadInt(state.LSFreq()); !ok || int(v) != cfg.LS.Frequency {
		state.WriteInts(state.LSFreq(), int64(cfg.LS.Frequency))
	}
}

func maybeGossip(lsArgs []string) {
	defer func() { recover() }() // a bug here must never show up after ls
	cfg, err := config.Load()
	if err != nil {
		return
	}
	syncFreqCache(cfg)
	if state.Exists(state.LSOff()) {
		return
	}
	now := time.Now()
	if last, ok := state.ReadInt(state.LSLast()); ok && now.Unix()-last < int64(cfg.LS.CooldownSeconds) {
		return
	}
	target, ok := gossip.ParseLsArgs(lsArgs)
	if !ok {
		return
	}
	entries, err := gossip.Scan(target.Dir, target.ShowHidden, cfg.LS.MaxEntriesScanned)
	visible := len(entries)
	if target.ShowDotNames {
		visible += 2 // "." and ".." are in the listing too
	}
	if err != nil || visible < cfg.LS.MinEntries {
		return
	}
	r := newRand()
	c, ok := gossip.Choose(gossip.Candidates(entries, gossip.Env{Now: now, Rand: r, AppInstalled: gossip.InstalledApp}), r)
	if !ok {
		return
	}
	printGossip(c.Text, cfg.LS.Style)
	state.WriteInts(state.LSLast(), now.Unix())
}

func gossipNow(dir string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, "warning:", err)
	}
	entries, err := gossip.Scan(dir, false, cfg.LS.MaxEntriesScanned)
	if err != nil {
		return fail("ls-gossip", err)
	}
	r := newRand()
	c, ok := gossip.Choose(gossip.Candidates(entries, gossip.Env{Now: time.Now(), Rand: r, AppInstalled: gossip.InstalledApp}), r)
	if !ok {
		printGossip("Nothing to report. It's suspiciously quiet in here.", cfg.LS.Style)
		return 0
	}
	printGossip(c.Text, cfg.LS.Style)
	return 0
}

func printGossip(text, style string) {
	line := "  ~ " + text
	if style == "dim" {
		line = "\033[2m" + line + "\033[0m"
	}
	fmt.Fprintf(stdout, "\n%s\n", line)
}
