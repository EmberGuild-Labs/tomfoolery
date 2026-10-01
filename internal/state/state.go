// Package state holds the file locations tomfoolery uses and small helpers
// for reading and writing the marker and state files kept there.
package state

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Label is the launchd label for the uptime-brag LaunchAgent.
const Label = "com.emberguildlabs.tomfoolery.uptime-brag"

// Home returns the user's home directory.
func Home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func ConfigDir() string  { return filepath.Join(Home(), ".config", "tomfoolery") }
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.toml") }
func HookFile() string   { return filepath.Join(ConfigDir(), "hook.zsh") }
func Dir() string        { return filepath.Join(Home(), ".local", "state", "tomfoolery") }
func BragDir() string    { return filepath.Join(Dir(), "uptime-brag") }
func KillDir() string    { return filepath.Join(Dir(), "kill") }
func LogDir() string     { return filepath.Join(Home(), "Library", "Logs", "tomfoolery") }
func LogFile() string    { return filepath.Join(LogDir(), "uptime-brag.log") }
func PlistFile() string {
	return filepath.Join(Home(), "Library", "LaunchAgents", Label+".plist")
}

// Marker and state files.
func LSOff() string         { return filepath.Join(Dir(), "ls-gossip.off") }
func LSFreq() string        { return filepath.Join(Dir(), "ls-gossip.freq") }
func LSLast() string        { return filepath.Join(Dir(), "ls-gossip.last") }
func KillOff() string       { return filepath.Join(Dir(), "kill-eulogy.off") }
func BragDisabled() string  { return filepath.Join(Dir(), "uptime-brag.disabled") }
func LastShownBoot() string { return filepath.Join(BragDir(), "last-shown-boot") }
func LastSeen() string      { return filepath.Join(BragDir(), "last-seen") }
func Record() string        { return filepath.Join(BragDir(), "record") }
func Funeral() string       { return filepath.Join(BragDir(), "funeral") }
func PidFile() string       { return filepath.Join(BragDir(), "brag.pid") }
func CommandFile() string   { return filepath.Join(BragDir(), "brag.command") }

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Touch creates path (and its parent directories) if it does not exist.
func Touch(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// Remove deletes path, ignoring "does not exist".
func Remove(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Write atomically replaces path with content, creating parent directories.
func Write(path, content string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ReadInts reads whitespace-separated integers from path.
func ReadInts(path string) ([]int64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var out []int64
	for _, f := range strings.Fields(string(b)) {
		n, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, len(out) > 0
}

// ReadInt reads a single integer from path.
func ReadInt(path string) (int64, bool) {
	v, ok := ReadInts(path)
	if !ok {
		return 0, false
	}
	return v[0], true
}

// WriteInts writes integers separated by spaces, with a trailing newline.
func WriteInts(path string, vals ...int64) error {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return Write(path, strings.Join(parts, " ")+"\n", 0o644)
}
