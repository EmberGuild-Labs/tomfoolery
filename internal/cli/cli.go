// Package cli implements the commands. One binary answers to four names:
// tomfoolery, ls-gossip, kill-with-eulogy and uptime-brag.
package cli

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EmberGuild-Labs/tomfoolery/internal/config"
	"github.com/EmberGuild-Labs/tomfoolery/internal/intro"
	"github.com/EmberGuild-Labs/tomfoolery/internal/shell"
	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

// Version is set at build time with -ldflags "-X .../cli.Version=...".
var Version = "dev"

// Tools are the names the binary is symlinked under.
var Tools = []string{"ls-gossip", "kill-with-eulogy", "uptime-brag"}

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func newRand() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }

// Main dispatches on the name the binary was invoked as and returns an exit code.
func Main(argv []string) int {
	name := filepath.Base(argv[0])
	args := argv[1:]
	switch name {
	case "ls-gossip":
		return lsGossip(args)
	case "kill-with-eulogy":
		return killWithEulogy(args)
	case "uptime-brag":
		return uptimeBrag(args)
	}
	return tomfoolery(args)
}

func tomfoolery(args []string) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}
	switch args[0] {
	case "ls-gossip":
		return lsGossip(args[1:])
	case "kill-with-eulogy":
		return killWithEulogy(args[1:])
	case "uptime-brag":
		return uptimeBrag(args[1:])
	case "install":
		return install(args[1:])
	case "uninstall":
		return uninstall(args[1:])
	case "status":
		return status()
	case "intro":
		if !intro.Play() {
			fmt.Fprintln(stderr, "tomfoolery intro: needs an interactive terminal")
			return 1
		}
		return 0
	case "hook":
		cfg, _ := config.Load()
		fmt.Fprint(stdout, shell.Render(mainBinary(), cfg.Kill.Wrap))
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "tomfoolery", Version)
		return 0
	case "help", "--help", "-h":
		usage(stdout)
		return 0
	}
	fmt.Fprintf(stderr, "tomfoolery: unknown command %q\n\n", args[0])
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprint(w, `tomfoolery: deliberately absurd, deliberately polite command-line toys.

Usage:
  tomfoolery install            one-time setup (symlinks, shell hook, LaunchAgent)
  tomfoolery uninstall [--yes]  remove everything install added
  tomfoolery status             show the state of all three tools
  tomfoolery intro              play the EmberGuild Labs intro (any key skips)
  tomfoolery version

  ls-gossip on|off|status|now [dir]|freq <0-100>
  kill-with-eulogy on|off|status
  uptime-brag on|off|show|status

Bypass any wrapper with: command ls, builtin kill
`)
}

func status() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, "warning:", err)
	}
	fmt.Fprintf(stdout, "tomfoolery %s\n", Version)
	hooked := shell.HasBlock(filepath.Join(state.Home(), ".zshrc")) || shell.HasBlock(filepath.Join(state.Home(), ".bash_profile"))
	fmt.Fprintf(stdout, "  shell hook        %s\n", map[bool]string{true: "installed", false: "not installed (run: tomfoolery install)"}[hooked && state.Exists(state.HookFile())])
	fmt.Fprintf(stdout, "  ls-gossip         %s, %d%% chance\n", onOff(!state.Exists(state.LSOff())), cfg.LS.Frequency)
	fmt.Fprintf(stdout, "  kill-with-eulogy  %s, wraps %s\n", onOff(!state.Exists(state.KillOff())), strings.Join(cfg.Kill.Wrap, ", "))
	fmt.Fprintf(stdout, "  uptime-brag       %s\n", bragSummary())
	return 0
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// mainBinary returns the path the hook and LaunchAgent should call. When
// invoked through one of the tool symlinks it resolves to the real binary.
func mainBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return "tomfoolery"
	}
	if filepath.Base(exe) != "tomfoolery" {
		if r, err := filepath.EvalSymlinks(exe); err == nil {
			exe = r
		}
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	return exe
}

func setMarker(path string, present bool) error {
	if present {
		return state.Touch(path)
	}
	return state.Remove(path)
}

func fail(tool string, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", tool, err)
	return 1
}
