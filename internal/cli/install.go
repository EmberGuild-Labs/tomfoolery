package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/EmberGuild-Labs/tomfoolery/internal/agent"
	"github.com/EmberGuild-Labs/tomfoolery/internal/brag"
	"github.com/EmberGuild-Labs/tomfoolery/internal/config"
	"github.com/EmberGuild-Labs/tomfoolery/internal/shell"
	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

// rcFile picks the shell startup file: ~/.zshrc, or ~/.bash_profile for bash.
func rcFile() string {
	if strings.HasSuffix(os.Getenv("SHELL"), "bash") {
		return filepath.Join(state.Home(), ".bash_profile")
	}
	return filepath.Join(state.Home(), ".zshrc")
}

func step(format string, a ...any) {
	fmt.Fprintf(stdout, "  • "+format+"\n", a...)
}

func install(args []string) int {
	intro := ""
	for _, a := range args {
		switch a {
		case "--intro":
			intro = "always"
		case "--no-intro":
			intro = "never"
		default:
			fmt.Fprintf(stderr, "tomfoolery install: unknown option %q\n", a)
			return 2
		}
	}
	firstInstall := !state.Exists(state.HookFile())
	// The EmberGuild Labs intro plays once, on the very first setup.
	if intro == "always" || (intro == "" && firstInstall) {
		playIntro()
	}

	bin := mainBinary()
	dir := filepath.Dir(bin)
	fmt.Fprintf(stdout, "Setting up tomfoolery (%s)\n", bin)

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Fprintln(stderr, "warning:", cfgErr)
	}

	// 1. Symlinks next to the main binary.
	step("Linking %s next to %s", strings.Join(Tools, ", "), filepath.Base(bin))
	for _, t := range Tools {
		link := filepath.Join(dir, t)
		if cur, err := os.Readlink(link); err == nil {
			if cur == bin || cur == filepath.Base(bin) {
				continue
			}
			os.Remove(link)
		} else if state.Exists(link) {
			fmt.Fprintf(stderr, "    skipped %s: a file that is not our symlink is already there\n", link)
			continue
		}
		if err := os.Symlink(bin, link); err != nil {
			return fail("tomfoolery install", fmt.Errorf("symlink %s: %w (try installing to a directory you own)", link, err))
		}
	}

	// 2. Static hook file.
	step("Writing %s", tilde(state.HookFile()))
	if err := state.Write(state.HookFile(), shell.Render(bin, cfg.Kill.Wrap), 0o644); err != nil {
		return fail("tomfoolery install", err)
	}
	if !state.Exists(state.ConfigFile()) {
		step("Writing default settings to %s", tilde(state.ConfigFile()))
		if err := state.Write(state.ConfigFile(), config.DefaultFileText, 0o644); err != nil {
			return fail("tomfoolery install", err)
		}
	}
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		return fail("tomfoolery install", err)
	}
	syncFreqCache(cfg)

	// 3. One marked block in the shell rc file.
	rc := rcFile()
	if shell.HasBlock(rc) {
		step("%s already sources the hook", tilde(rc))
	} else {
		step("Adding a marked block to %s that sources the hook", tilde(rc))
		if _, err := shell.AddBlock(rc); err != nil {
			return fail("tomfoolery install", err)
		}
	}
	warnAliases(rc)

	// 4. LaunchAgent plist, written but not loaded until `uptime-brag on`.
	step("Writing the uptime-brag LaunchAgent to %s (not loaded)", tilde(state.PlistFile()))
	if err := agent.WritePlist(bin); err != nil {
		return fail("tomfoolery install", err)
	}
	if !firstInstall && agent.Loaded() {
		// An upgrade may have moved the binary; reload so launchd sees the new path.
		agent.Unload()
		agent.Load()
	}

	if firstInstall {
		// uptime-brag starts off; the others start on.
		state.Touch(state.BragDisabled())
	}
	if boot, err := brag.BootTime(); err == nil {
		brag.PrepareFuneral(boot, nowFn())
	}

	fmt.Fprintln(stdout, "\nDone. Open a new terminal tab (or run: source ~/.config/tomfoolery/hook.zsh), then:")
	fmt.Fprintln(stdout, "  ls-gossip now        # see some gossip immediately")
	fmt.Fprintln(stdout, "  uptime-brag on       # opt in to the brag window")
	fmt.Fprintln(stdout, "  tomfoolery status")
	return 0
}

var aliasRe = regexp.MustCompile(`(?m)^\s*alias\s+(ls|kill)=(['"]?)(\S+)`)

// warnAliases flags aliases that would route around the wrappers. An alias
// like ls='ls -G' is fine (it still calls the ls function); one that points
// at a different program, like ls='eza', bypasses it.
func warnAliases(rc string) {
	b, err := os.ReadFile(rc)
	if err != nil {
		return
	}
	for _, m := range aliasRe.FindAllStringSubmatch(string(b), -1) {
		name, target := m[1], strings.Trim(m[3], `'"`)
		if target != name {
			fmt.Fprintf(stderr, "    note: %s has alias %s=%s..., which bypasses the %s wrapper\n", tilde(rc), name, target, name)
		}
	}
}

func uninstall(args []string) int {
	yes := len(args) > 0 && (args[0] == "--yes" || args[0] == "-y")
	bin := mainBinary()
	fmt.Fprintln(stdout, "Removing tomfoolery")

	step("Unloading the uptime-brag LaunchAgent and removing its plist")
	if err := agent.Unload(); err != nil {
		fmt.Fprintln(stderr, "    warning:", err)
	}
	state.Remove(state.PlistFile())

	if brag.Stop() {
		step("Stopped the running brag window")
	}

	for _, rc := range []string{filepath.Join(state.Home(), ".zshrc"), filepath.Join(state.Home(), ".bash_profile")} {
		removed, err := shell.RemoveBlock(rc)
		if err != nil {
			fmt.Fprintln(stderr, "    warning:", err)
		} else if removed {
			step("Removed the tomfoolery block from %s", tilde(rc))
		}
	}

	step("Deleting %s", tilde(state.HookFile()))
	state.Remove(state.HookFile())

	dir := filepath.Dir(bin)
	for _, t := range Tools {
		link := filepath.Join(dir, t)
		if cur, err := os.Readlink(link); err == nil && (cur == bin || cur == filepath.Base(bin)) {
			step("Removing symlink %s", link)
			os.Remove(link)
		}
	}

	keep := !yes
	if !yes && isTerminal(os.Stdin) {
		rec := brag.Record()
		fmt.Fprintf(stdout, "\nAlso delete your settings and state (%s, %s)", tilde(state.ConfigDir()), tilde(state.Dir()))
		if rec > 0 {
			fmt.Fprintf(stdout, ",\nincluding your uptime record")
		}
		fmt.Fprint(stdout, "? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		keep = !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
	}
	if keep {
		fmt.Fprintf(stdout, "Kept %s and %s.\n", tilde(state.ConfigDir()), tilde(state.Dir()))
	} else {
		os.RemoveAll(state.ConfigDir())
		os.RemoveAll(state.Dir())
		os.RemoveAll(state.LogDir())
		step("Deleted settings, state and logs")
	}
	fmt.Fprintf(stdout, "\nDone. The binary itself is still at %s; delete it if you like.\n", bin)
	fmt.Fprintln(stdout, "Open a new terminal tab so the ls and kill wrappers are gone.")
	return 0
}

func tilde(p string) string {
	if h := state.Home(); strings.HasPrefix(p, h+"/") {
		return "~" + p[len(h):]
	}
	return p
}
