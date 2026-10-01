package cli

import (
	"fmt"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/agent"
	"github.com/EmberGuild-Labs/tomfoolery/internal/brag"
	"github.com/EmberGuild-Labs/tomfoolery/internal/config"
	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
	"github.com/EmberGuild-Labs/tomfoolery/internal/words"
)

var nowFn = time.Now

func uptimeBrag(args []string) int {
	if len(args) == 0 {
		args = []string{"status"}
	}
	cfg, cfgErr := config.Load()
	switch args[0] {
	case "--launch":
		return launch(cfg)
	case "--loop":
		if err := brag.Loop(cfg); err != nil {
			return fail("uptime-brag", err)
		}
		return 0
	case "on":
		if err := state.Remove(state.BragDisabled()); err != nil {
			return fail("uptime-brag", err)
		}
		if err := agent.WritePlist(mainBinary()); err != nil {
			return fail("uptime-brag", err)
		}
		if err := agent.Load(); err != nil {
			return fail("uptime-brag", err)
		}
		fmt.Fprintln(stdout, "uptime-brag is on. The window opens once per boot, at login.")
		return 0
	case "off":
		if err := state.Touch(state.BragDisabled()); err != nil {
			return fail("uptime-brag", err)
		}
		if err := agent.Unload(); err != nil {
			fmt.Fprintln(stderr, "warning:", err)
		}
		stopped := brag.Stop()
		msg := "uptime-brag is off."
		if stopped {
			msg += " Closed the running window's loop."
		}
		fmt.Fprintln(stdout, msg)
		return 0
	case "show":
		boot, err := brag.BootTime()
		if err != nil {
			return fail("uptime-brag", err)
		}
		if pid := brag.RunningPID(); pid != 0 {
			fmt.Fprintf(stdout, "uptime-brag is already open (PID %d).\n", pid)
			return 0
		}
		brag.PrepareFuneral(boot, nowFn())
		state.WriteInts(state.LastShownBoot(), boot)
		if err := brag.OpenWindow(mainBinary(), cfg); err != nil {
			return fail("uptime-brag", err)
		}
		return 0
	case "status":
		if cfgErr != nil {
			fmt.Fprintln(stderr, "warning:", cfgErr)
		}
		fmt.Fprintln(stdout, "uptime-brag is "+bragSummary()+".")
		return 0
	}
	fmt.Fprintf(stderr, "uptime-brag: unknown command %q (try on, off, show, status)\n", args[0])
	return 2
}

func bragSummary() string {
	s := onOff(!state.Exists(state.BragDisabled()))
	if agent.Loaded() {
		s += ", agent loaded"
	} else {
		s += ", agent not loaded"
	}
	if pid := brag.RunningPID(); pid != 0 {
		s += fmt.Sprintf(", window open (PID %d)", pid)
	}
	if boot, err := brag.BootTime(); err == nil {
		s += ", up " + words.LongDuration(nowFn().Sub(time.Unix(boot, 0)))
		if brag.ShownThisBoot(boot) {
			s += ", already shown this boot"
		}
	}
	if rec := brag.Record(); rec > 0 {
		s += ", record " + words.LongDuration(rec)
	}
	return s
}

// launch is what the LaunchAgent runs at login. It opens the window at most
// once per boot.
func launch(cfg config.Config) int {
	logf := func(format string, a ...any) {
		fmt.Fprintf(stdout, "%s "+format+"\n", append([]any{nowFn().Format("2006-01-02 15:04:05")}, a...)...)
	}
	if state.Exists(state.BragDisabled()) {
		logf("disabled; not opening")
		return 0
	}
	boot, err := brag.BootTime()
	if err != nil {
		logf("error: %v", err)
		return 1
	}
	if brag.ShownThisBoot(boot) {
		logf("already shown for boot %d; not opening", boot)
		return 0
	}
	if err := brag.PrepareFuneral(boot, nowFn()); err != nil {
		logf("warning: funeral: %v", err)
	}
	if err := state.WriteInts(state.LastShownBoot(), boot); err != nil {
		logf("error: %v", err)
		return 1
	}
	if pid := brag.RunningPID(); pid != 0 {
		logf("window already running (PID %d)", pid)
		return 0
	}
	if err := brag.OpenWindow(mainBinary(), cfg); err != nil {
		logf("error: %v", err)
		return 1
	}
	logf("opened window for boot %d", boot)
	return 0
}
