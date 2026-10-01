# tomfoolery

Three deliberately absurd, deliberately polite command-line toys for macOS.

| Command | What it does |
|---|---|
| `ls-gossip` | Your normal `ls` output, followed occasionally by one line of speculation about the files in the directory |
| `kill-with-eulogy` | Your normal `kill`, followed by a short eulogy for the process that just died |
| `uptime-brag` | A small Terminal window, opened once per boot, that brags about how long your Mac has been up |

> **Status: v0.1.0, working.** All three tools are implemented, tested, and
> installed on the dev Mac. See [Notes and decisions](#notes-and-decisions) for
> where the build deviates from the original spec.

---

## Table of contents

1. [Design principles](#design-principles)
2. [Requirements](#requirements)
3. [Install](#install)
4. [Quick start](#quick-start)
5. [ls-gossip](#ls-gossip)
6. [kill-with-eulogy](#kill-with-eulogy)
7. [uptime-brag](#uptime-brag)
8. [Configuration](#configuration)
9. [Files and locations](#files-and-locations)
10. [How it works](#how-it-works)
11. [Privacy](#privacy)
12. [Uninstall](#uninstall)
13. [Troubleshooting](#troubleshooting)
14. [Known limitations](#known-limitations)
15. [Roadmap](#roadmap)
16. [Development](#development)
17. [Notes and decisions](#notes-and-decisions)
18. [License](#license)

---

## Design principles

These apply to all three tools and are treated as requirements, not suggestions.

1. **Never change real behavior.** The underlying command always runs first (or, for `kill`, runs with identical arguments), and its output and exit code are preserved exactly. The joke is only ever additional output.
2. **Interactive terminals only.** Wrappers are shell functions defined in interactive shells. Scripts, cron jobs, pipes, `xargs`, and redirects are never affected.
3. **Easy to turn off.** Every tool has an `on` / `off` command, and the setting applies to all open terminals immediately.
4. **Always an escape hatch.** `command ls` and `builtin kill` always bypass the wrappers.
5. **Quiet by default.** Gossip is rate-limited, eulogies only appear for confirmed deaths, and the brag window never steals focus.
6. **Cheap.** No background daemon for `ls-gossip` or `kill-with-eulogy`. `uptime-brag` is a sleeping process that wakes once per minute.
7. **Local only.** No network access, no telemetry, no file contents read.

---

## Requirements

- macOS 13 (Ventura) or later, Apple Silicon or Intel
- zsh (the macOS default shell). bash 3.2 (also shipped with macOS) is supported on a best-effort basis
- Terminal.app (default) or iTerm2 for `uptime-brag`
- Building from source: Go 1.22 or later

---

## Install

One line, no Go or Xcode needed:

```sh
curl -fsSL https://raw.githubusercontent.com/EmberGuild-Labs/tomfoolery/main/install.sh | sh
```

The script downloads the prebuilt universal (Apple Silicon + Intel) binary from the latest [GitHub release](https://github.com/EmberGuild-Labs/tomfoolery/releases), checks its SHA-256, installs it to `~/.local/bin` (no sudo), and runs `tomfoolery install` (described below), with the intro. Read it first if you like: [install.sh](install.sh). Options are environment variables placed after the pipe, e.g. `| TOMFOOLERY_NO_INTRO=1 sh`:

| Variable | Effect |
|---|---|
| `TOMFOOLERY_PREFIX=/usr/local` | Install to `$PREFIX/bin` instead of `~/.local/bin` |
| `TOMFOOLERY_VERSION=v0.1.1` | Install a specific release instead of the latest |
| `TOMFOOLERY_NO_INTRO=1` | Skip the EmberGuild Labs intro |
| `TOMFOOLERY_NO_SETUP=1` | Install the binary only; run `tomfoolery install` yourself later |

Or download `tomfoolery-darwin-universal.tar.gz` from the releases page by hand, put `tomfoolery` somewhere on your PATH, and run `tomfoolery install`. A file downloaded in a browser is quarantined by Gatekeeper, because the binary is not notarized. Clear that with `xattr -d com.apple.quarantine tomfoolery`. The curl installer does not have this problem.

### From source

Homebrew tap is planned. Needs Go 1.22+ and the Xcode command line tools (for `lipo`):

```sh
git clone https://github.com/EmberGuild-Labs/tomfoolery.git
cd tomfoolery
make build                          # universal (arm64 + x86_64) binary in ./build
make install PREFIX=$HOME/.local    # ~/.local/bin, no sudo (what this Mac uses)
# or: sudo make install             # /usr/local/bin
```

Then run the one-time setup:

```sh
tomfoolery install
```

`tomfoolery install` does the following, and prints each step before doing it:

0. On a first install only, plays the EmberGuild Labs studio intro, which is built into the binary (about 10 seconds; any key or Ctrl-C skips it). Use `--intro` to force it on a reinstall, and `--no-intro` (or `TOMFOOLERY_NO_INTRO=1`) to skip it. It needs a terminal of at least 64×14; a smaller one shows a resize hint and times out after 14 seconds. It never plays without a terminal (scripts, CI), so it cannot block setup. Replay it any time with `tomfoolery intro`.
1. Creates symlinks `ls-gossip`, `kill-with-eulogy`, and `uptime-brag` next to the main binary. All four are the same executable, which dispatches on its own name.
2. Writes a static hook file to `~/.config/tomfoolery/hook.zsh`.
3. Appends one marked block to `~/.zshrc` that sources that file. Nothing is spawned at shell startup.
4. Writes the LaunchAgent plist for `uptime-brag` (but does not load it until you run `uptime-brag on`).
5. Writes a default `config.toml` if you do not have one. `ls-gossip` and `kill-with-eulogy` start **on**; `uptime-brag` starts **off**.

Open a new terminal tab afterward so the hook is loaded.

---

## Quick start

```sh
ls-gossip on
kill-with-eulogy on
uptime-brag on
```

Replay the EmberGuild Labs intro:

```sh
tomfoolery intro
```

Check status of everything at once:

```sh
tomfoolery status
```

Turn everything off without uninstalling:

```sh
ls-gossip off && kill-with-eulogy off && uptime-brag off
```

---

## ls-gossip

### What you see

```
$ ls
Desktop        Downloads      notes_final_v3.txt   package-lock.json
Documents      node_modules   package.json         Screenshot 2026-09-12 at 14.03.11.png

  ~ package.json and package-lock.json are definitely related. Neither will comment.
```

The listing is the real `ls` output. The gossip line is printed afterward, to stderr, dimmed.

### When gossip appears

All of the following must be true:

- `ls-gossip` is on
- stdout and stderr are both terminals (so never in pipes or redirects)
- `ls` exited with status 0
- exactly one directory is being listed (the current directory by default)
- the directory has at least `min_entries` visible entries (default 4)
- the random roll succeeds (default 35% chance per eligible run)
- the previous gossip was at least `cooldown_seconds` ago (default 20)

Flags are respected: gossip only concerns entries `ls` actually showed, so dotfiles are only gossiped about if you passed `-a` or `-A`.

### What it gossips about

Gossip is generated from metadata only: names, extensions, sizes, and modification times. File contents are never opened.

- Related pairs: `package.json` and `package-lock.json`, `foo.c` and `foo.o`, `x.zip` next to `x/`
- Version-suffix names: `final`, `final_v3`, `copy`, `old`, `backup`, `new`
- Piles: many `Screenshot ...` files, many `Untitled` files
- Leftovers: a `.dmg` next to an installed app's data
- Empty files (0 bytes)
- Oldest and newest file by modification time
- The largest file

### Commands

```sh
ls-gossip on          # enable (default state after install)
ls-gossip off         # disable
ls-gossip status      # prints on/off and current frequency
ls-gossip now         # force one gossip line for the current directory, ignoring the dice and cooldown
ls-gossip freq 20     # set the percent chance per eligible run (0-100)
```

Bypass for a single call: `command ls` or `\ls`.

### Performance

The on/off check is a file-existence test inside the shell function. When gossip is off, the cost is one `[[ -f ... ]]`. When on but not rolled, the cost is the same plus the dice roll. The gossip binary only runs when the roll succeeds, and scans at most `max_entries_scanned` entries (default 2000).

---

## kill-with-eulogy

### What you see

```
$ kill 4821
In memory of Slack Helper (PID 4821)
Born 2 days 3 hours ago. Consumed 41 minutes of CPU and 612 MB of memory.
Survived by 6 child processes.
Went peacefully (SIGTERM).
```

### How it works

1. Before sending the signal, the wrapper reads process metadata with `ps` (name, elapsed time, CPU time, resident memory, owner) and counts children with `pgrep -P`.
2. It calls `builtin kill` with your original arguments, unmodified.
3. If `kill` succeeded and the signal is one that normally terminates, it polls `kill -0` for up to `death_wait_ms` (default 1000) to confirm the process is gone.
4. If the process is gone, it prints the eulogy. If it is still alive, it prints a one-line note that the process has received the news and declined to die.
5. The original exit status is returned in every case.

### Tone by cause of death

| Signal | Flavor |
|---|---|
| `SIGTERM` (default) | Went peacefully |
| `SIGINT` | Interrupted mid-sentence |
| `SIGHUP` | Left when the terminal hung up |
| `SIGKILL` (`-9`) | Taken without warning |
| Survives `SIGTERM` | Informed, and has declined |

Extra lines appear for long-lived processes (older than 7 days get a longer eulogy) and for processes that used essentially no CPU time.

### What it deliberately does not touch

- `kill -0`, `kill -l`, and signals that do not terminate (`SIGUSR1`, `SIGSTOP`, and so on) pass straight through with no eulogy
- Job specs such as `kill %1` pass straight through
- Permission errors (`Operation not permitted`) pass straight through
- Argument forms the wrapper cannot parse with certainty pass straight through
- `/bin/kill`, `xargs kill`, and anything run from a script are not wrapped, because only your interactive shell has the function

### `pkill` and `killall`

Opt-in, best-effort. Enable in config:

```toml
[kill_eulogy]
wrap = ["kill", "pkill", "killall"]
```

The wrapper resolves targets with `pgrep` first, then eulogizes each process that actually died. Name-matching rules differ slightly between `killall` and `pgrep`, so edge cases may produce fewer eulogies than processes killed. The kill itself is never affected.

### Commands

```sh
kill-with-eulogy on
kill-with-eulogy off
kill-with-eulogy status
```

Bypass for a single call: `builtin kill 4821`.

---

## uptime-brag

### What you see

A small Terminal window (default 64 columns by 10 rows), opened in the background, redrawn once per minute:

```
  UPTIME BRAG

  Up since   Mon 14 Sep 09:12
  Uptime     16 days 4 hours 51 minutes
  Best       23 days 4 hours 12 minutes

  Sixteen days. Your software updates have not been this patient.
```

All times use the 24-hour clock.

### When the window opens

- Once per boot, at login, via a LaunchAgent with `RunAtLoad` and no `KeepAlive`
- **Not** when Terminal is quit and reopened
- **Not** after logging out and back in without rebooting (a per-boot marker prevents this)
- Manually, any time, with `uptime-brag show`

If you close the window, it stays closed until the next boot or until you run `uptime-brag show`.

### How "once per boot" works

At launch the agent reads the boot time:

```sh
sysctl -n kern.boottime
# { sec = 1789000000, usec = 123456 } Mon Sep 14 09:12:00 2026
```

It compares the `sec` value to the one stored in `last-shown-boot`. Because macOS can adjust the boot time slightly when the system clock is corrected, the comparison uses a tolerance of 120 seconds. If the values match, nothing opens.

### Window behavior

- Opened with `open -g -a Terminal brag.command`. The `-g` flag keeps it from stealing focus, and using a `.command` file means no Automation permission prompt (unlike driving Terminal with AppleScript).
- Resized on startup with the standard xterm resize escape sequence.
- The loop sleeps and redraws once per minute, aligned to the start of the minute, so it uses effectively no CPU.
- The script writes its PID to a pidfile so `uptime-brag off` can stop it cleanly.

### Brags

Lines are drawn from a pool tiered by uptime. Milestone days trigger a special line the first time they are crossed:

| Uptime | Tone |
|---|---|
| Under 1 hour | Fresh boot, nothing to brag about yet |
| 1 day | Mildly pleased |
| 3, 7, 14 days | Increasingly smug |
| 30, 60, 100 days | Openly insufferable |
| 365 days | Demands a plaque |

### Streak funeral

Every 5 minutes the script records the current uptime in `last-seen`. On the next boot, the first brag window opens with a mock funeral for the previous streak, using the last recorded value (so the length is approximate, and may be up to 5 minutes short after a crash or power loss). If the streak beat your record, it is saved to `record`.

### Commands

```sh
uptime-brag on        # load the LaunchAgent and enable
uptime-brag off       # unload the LaunchAgent, stop the running window, mark disabled
uptime-brag show      # open the window now, regardless of the per-boot marker
uptime-brag status    # enabled/disabled, window running or not, current uptime, record
```

### macOS notes

- On macOS 13 and later, installing a LaunchAgent adds an entry under System Settings > General > Login Items & Extensions ("Allow in the Background"). This is expected. `uptime-brag off` unloads it but leaves the toggle entry until you run `tomfoolery uninstall`.
- Uptime is computed from the boot time and **includes time spent asleep**. This is the same number `uptime` reports, so a laptop that sleeps a lot will have a generous streak.

---

## Configuration

Optional. Located at `~/.config/tomfoolery/config.toml`. Defaults shown.

```toml
[ls_gossip]
frequency = 35              # percent chance per eligible run
cooldown_seconds = 20
min_entries = 4
max_entries_scanned = 2000
style = "dim"               # "dim" or "plain"

[kill_eulogy]
wrap = ["kill"]             # add "pkill" and/or "killall"
death_wait_ms = 1000        # max wait for the process to disappear

[uptime_brag]
terminal = "Terminal"       # "Terminal" or "iTerm"
focus = false               # true = bring window to front when opened
window_cols = 64
window_rows = 10
redraw_seconds = 60
last_seen_interval_minutes = 5
milestones_days = [1, 3, 7, 14, 30, 60, 100, 365]
```

Config changes to `wrap` require opening a new shell (or `source ~/.config/tomfoolery/hook.zsh`). Everything else is read at use time.

---

## Files and locations

| Path | Purpose |
|---|---|
| `~/.config/tomfoolery/config.toml` | Optional configuration |
| `~/.config/tomfoolery/hook.zsh` | Static shell hook sourced from `~/.zshrc` |
| `~/.local/state/tomfoolery/ls-gossip.off` | Present means `ls-gossip` is off |
| `~/.local/state/tomfoolery/kill-eulogy.off` | Present means `kill-with-eulogy` is off |
| `~/.local/state/tomfoolery/uptime-brag.disabled` | Present means `uptime-brag` is off |
| `~/.local/state/tomfoolery/ls-gossip.freq` | Cached frequency the hook reads for its dice roll (kept in sync with `config.toml`) |
| `~/.local/state/tomfoolery/ls-gossip.last` | Time of the last gossip, for the cooldown |
| `~/.local/state/tomfoolery/kill/` | Short-lived process snapshots between the capture and the eulogy |
| `~/.local/state/tomfoolery/uptime-brag/funeral` | Pending streak funeral for the next window |
| `~/.local/state/tomfoolery/uptime-brag/brag.command` | Script Terminal opens to run the window |
| `~/.local/state/tomfoolery/uptime-brag/last-shown-boot` | Boot time (epoch seconds) of the last boot a window was opened for |
| `~/.local/state/tomfoolery/uptime-brag/last-seen` | Most recent uptime sample, for the streak funeral |
| `~/.local/state/tomfoolery/uptime-brag/record` | Longest recorded streak |
| `~/.local/state/tomfoolery/uptime-brag/brag.pid` | PID of the running brag loop |
| `~/Library/LaunchAgents/com.emberguildlabs.tomfoolery.uptime-brag.plist` | LaunchAgent definition |
| `~/Library/Logs/tomfoolery/uptime-brag.log` | Agent stdout/stderr |

---

## How it works

**One binary, three names.** `tomfoolery` is a single Go executable. `ls-gossip`, `kill-with-eulogy`, and `uptime-brag` are symlinks to it, and it dispatches on `argv[0]`. The management commands (`on`, `off`, `status`) live in the same binary.

**Shell hook.** `hook.zsh` defines two functions, `ls` and `kill`, only when the shell is interactive. Sketch of the `ls` wrapper:

```zsh
function ls {
  command ls "$@"
  local rc=$?
  if [ $rc -eq 0 ] && [ -t 1 ] && [ -t 2 ] && [ ! -f "$_tomfoolery_state/ls-gossip.off" ]; then
    local freq=35
    [ -r "$_tomfoolery_state/ls-gossip.freq" ] && freq=$(<"$_tomfoolery_state/ls-gossip.freq")
    if [ $(( RANDOM % 100 )) -lt "$freq" ]; then
      "$_tomfoolery_bin" ls-gossip --maybe -- "$@" >&2
    fi
  fi
  return $rc
}
```

The dice roll happens in the shell, so the binary only runs when the roll succeeds. `$(<file)` is a zsh builtin read, so even the frequency lookup spawns nothing.

The `kill` wrapper runs `tomfoolery kill-with-eulogy --capture kill -- "$@"`, which parses the arguments and, only if it is certain they name specific PIDs with a terminating signal, snapshots them with `ps`/`pgrep` into a temp file and prints its path. Then `builtin kill "$@"` runs unchanged in the current shell, so job specs still work. If it succeeded, `--after <snapshot>` polls `kill -0` (and checks for zombies) for up to `death_wait_ms`, and prints the eulogies to stderr. Inside a function, zsh prefixes builtin errors with the function name and line number (`kill:7: ...`), so the wrapper rewrites that prefix back to the usual `kill: `, and kill's own messages look exactly as they would without it.

**Why a static hook file instead of `eval "$(tomfoolery init zsh)"`.** The `eval` form spawns a process every time a shell starts. Sourcing a pre-generated file costs nothing. `tomfoolery install` regenerates the file on upgrade.

**Gossip engine.** Template-based and fully offline. It builds a small candidate set from directory metadata (related pairs, suffix patterns, piles, extremes), picks one at random, and fills a template. No model calls, no network.

**Brag agent.** The LaunchAgent runs a short launcher that checks the disabled marker and the per-boot marker, and if both pass, runs `open -g -a <terminal> brag.command`. The `.command` file runs the redraw loop.

---

## Privacy

- No network access of any kind and no telemetry
- `ls-gossip` reads directory entries and their metadata (name, size, modification time). It never opens files
- `kill-with-eulogy` reads process metadata from `ps` and `pgrep` for the process you are killing
- `uptime-brag` reads the boot time and stores a few numbers locally
- All state lives under `~/.config/tomfoolery` and `~/.local/state/tomfoolery`

---

## Uninstall

```sh
tomfoolery uninstall
```

This unloads the LaunchAgent and removes its plist, stops any running brag window, removes the marked block from `~/.zshrc`, deletes `hook.zsh`, and removes the symlinks. It asks before deleting the state directory, since that holds your uptime record.

Manual fallback if the binary is already gone:

```sh
launchctl bootout gui/$(id -u)/com.emberguildlabs.tomfoolery.uptime-brag
rm ~/Library/LaunchAgents/com.emberguildlabs.tomfoolery.uptime-brag.plist
# then delete the "tomfoolery" block from ~/.zshrc
rm -rf ~/.config/tomfoolery ~/.local/state/tomfoolery
```

---

## Troubleshooting

**`ls-gossip` is on but I never see gossip.**
First, terminal tabs that were already open during `tomfoolery install` don't have the hook. Open a new tab or run `source ~/.zshrc`, and check that `type ls` reports a shell function. Beyond that, gossip is quiet by default: a 35% roll, a 20-second cooldown, and at least 4 entries (see [When gossip appears](#when-gossip-appears)). Run `ls-gossip now` to force one.

**I want gossip on every `ls`.**
Set these in `~/.config/tomfoolery/config.toml` (or run `ls-gossip freq 100` for the first one):

```toml
[ls_gossip]
frequency = 100
cooldown_seconds = 0
min_entries = 1
```

Every directory with at least one entry has something to say. When nothing specific stands out, a fallback line about the residents is used.

**I have an `ls` alias.**
The hook defines its wrappers with `function ls { ... }` rather than `ls() { ... }`, so an existing alias cannot break the definition. An alias such as `alias ls='ls -G'` keeps working and still goes through the wrapper. An alias to a different program (for example `alias ls='eza'`) bypasses the wrapper, and `tomfoolery install` prints a note when it sees one.

**Terminal asks "Do you want to terminate running processes in this window?" when I close the brag window.**
That is Terminal's default for windows running anything other than a shell. Click Terminate, or change it under Terminal > Settings > Profiles > Shell > Ask before closing.

**`kill` completion stopped working.**
It should not. zsh completion is keyed to the command name, not to whether it is a builtin or function. If it did break, run `compinit` again and report an issue.

**The brag window did not open at login.**
Check `uptime-brag status` (enabled?), then `~/Library/Logs/tomfoolery/uptime-brag.log`. Confirm the agent is loaded with `launchctl print gui/$(id -u)/com.emberguildlabs.tomfoolery.uptime-brag`. Also check that the toggle is allowed under System Settings > General > Login Items & Extensions.

**The brag window did not open, and status says it already showed this boot.**
That is the per-boot guard working. Use `uptime-brag show` to open it manually.

**An empty Terminal window appears after I restart Terminal.**
Terminal.app's window restoration can bring back a closed window as a plain shell. Turn off restoration for Terminal in System Settings > Desktop & Dock ("Close windows when quitting an application"), or close the window and quit Terminal with it empty.

**I need the real command, right now.**
`command ls`, `builtin kill`, or open a shell with `zsh -f` (skips `.zshrc`).

---

## Known limitations

- zsh is the primary target. bash 3.2 is best-effort. fish is not supported yet
- `kill-with-eulogy` only sees kills made from your interactive shell
- Uptime includes sleep time, matching the system `uptime`
- The streak funeral length can be up to 5 minutes short after a crash
- `killall` name matching differs slightly from `pgrep`, so some kills may get no eulogy

---

## Roadmap

- v0.1: `ls-gossip` and `kill-with-eulogy` on zsh, toggle commands, config file
- v0.2: `uptime-brag` with per-boot guard, milestones, streak funeral
- v0.3: iTerm2 support for `uptime-brag`, optional `pkill` and `killall` wrapping
- v0.4: per-directory gossip memory ("someone new moved in")
- Later: fish support, user-supplied gossip and eulogy template files

---

## Releasing

Tag, build the release artifacts, and publish them. `install.sh` always picks up the latest release.

```sh
git tag v0.2.0 && git push origin v0.2.0
make dist                       # dist/tomfoolery-darwin-universal.tar.gz + dist/SHA256SUMS
gh release create v0.2.0 dist/* --title v0.2.0 --generate-notes
```

## Development

```sh
make build        # universal binary into ./build
make test         # unit tests (gossip generator, signal parsing, boot-time guard)
make lint         # go vet + staticcheck
make dev-install  # symlinks the build into your PATH and writes the hook, for local testing
```

Layout (planned):

Layout:

```
cmd/tomfoolery/      entry point
internal/cli/        argv[0] dispatch and every command (install, status, on/off, ...)
internal/gossip/     ls argument parsing, directory scan, candidates and templates
internal/eulogy/     signal parsing (kill, pkill, killall), process capture, eulogies
internal/brag/       boot-time guard, brag lines, milestones, funeral, window loop
internal/agent/      LaunchAgent plist and launchctl bootstrap/bootout
internal/intro/      EmberGuild Labs studio intro (Go port of eglabs.py)
internal/shell/      hook template and the marked ~/.zshrc block
internal/config/     config.toml loading (small built-in TOML subset parser)
internal/state/      file locations and state-file helpers
internal/words/      durations, sizes, spelled-out numbers
```

The tests cover signal-argument parsing (anything ambiguous must pass through), the boot-time tolerance and funeral, gossip candidates and `ls` argument parsing on fixture directories, config parsing, and the `~/.zshrc` block round trip. Run `make test`.

To try it without touching your real setup, point `HOME` at a scratch directory: `HOME=/tmp/tf ./build/tomfoolery install --no-intro`.

---

## Notes and decisions

Decisions made while building v0.1.0:

- **No dependencies.** Standard library only, including a small TOML-subset parser (tables, strings, integers, booleans, arrays). A typo in `config.toml` is reported, and defaults are used.
- **Installed to `~/.local/bin`** on this Mac (`make install PREFIX=$HOME/.local`), because that directory is already on PATH and needs no sudo. The original `~/.zshrc` was backed up to `~/.zshrc.pre-tomfoolery` before setup.
- **Studio intro on first setup, built in.** The EmberGuild Labs intro started as `eglabs.py` in the separate "EGLabs Loading" project. It is ported line for line to Go in `internal/intro`, so the single binary carries it with no Python dependency. On a Mac without the developer tools, `python3` is a stub that pops an install dialog, which is a bad first impression for an installer. The port has the same timeline, heat palette, 5-row font, spiral, glint, crumble and particles, plus truecolor with a 256-color fallback (`EGLABS_COLOR=256`) and diffed frames inside synchronized-update escapes. It was checked against the Python original by rendering both to HTML at 2.0, 4.5, 5.6 and 8.0 s; the frames match apart from randomness. `go test ./internal/intro` plays the whole sequence headlessly at three sizes. Set `INTRO_SNAP=/path/out.html` to write snapshot frames.
- **`function ls` syntax** instead of `ls()`, which removes the alias conflict the original spec warned about.
- **Gossip and eulogies go to stderr**, so stdout is byte-for-byte the real command's output.
- **The dice roll lives in the shell.** `ls-gossip freq N` writes both `config.toml` (preserving comments) and a one-number cache file the hook reads. The binary re-syncs the cache from `config.toml` whenever it runs, so hand edits are picked up too.
- **Record is set at the funeral.** The `Best` line shows the larger of the saved record and the current streak. The saved record only changes when a streak ends, which is when the funeral compares it.
- **Milestone lines** show for the whole of the milestone day. Other brag lines rotate hourly, seeded by the boot, so the window doesn't flicker between lines on each redraw.
- **pkill/killall** are best-effort. `killall` is only wrapped in its simple form (`killall [-SIGNAL] name...`), and any other flag passes straight through. A single kill shows at most 5 full eulogies; the rest are summarized.
- **Kill snapshots** are refused if the path is outside `~/.local/state/tomfoolery/kill/`, and leftovers older than an hour are cleaned up.
- **Easy install for others:** a `curl | sh` script plus prebuilt release binaries, so people don't need Go or Xcode. When piped, the script re-attaches setup to `/dev/tty`, so the intro still plays and can be skipped with a key. Releases are published with `make dist` and `gh release create`. The binary is ad-hoc signed (Go does this for arm64) but not notarized, which only matters for browser downloads.
- **Public repo** at github.com/EmberGuild-Labs/tomfoolery, by explicit choice for this project (other EmberGuild Labs repos default to private).
- **Known rough edge:** Terminal asks for confirmation when you close the brag window, because a non-shell process is running in it (see Troubleshooting).

## License

MIT. See [LICENSE](LICENSE).
