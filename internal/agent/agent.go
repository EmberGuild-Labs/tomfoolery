// Package agent manages the uptime-brag LaunchAgent.
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/EmberGuild-Labs/tomfoolery/internal/state"
)

// RunAtLoad with no KeepAlive: launchd runs the launcher once per login,
// and the launcher's per-boot marker narrows that to once per boot.
const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>uptime-brag</string>
		<string>--launch</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// Plist renders the LaunchAgent definition for the binary at bin.
func Plist(bin string) string {
	return fmt.Sprintf(plistTemplate, state.Label, xmlEscape(bin),
		xmlEscape(state.LogFile()), xmlEscape(state.LogFile()))
}

// WritePlist writes the plist (it does not load it).
func WritePlist(bin string) error {
	if err := os.MkdirAll(state.LogDir(), 0o755); err != nil {
		return err
	}
	return state.Write(state.PlistFile(), Plist(bin), 0o644)
}

func domain() string  { return fmt.Sprintf("gui/%d", os.Getuid()) }
func service() string { return domain() + "/" + state.Label }

// Loaded reports whether launchd currently has the agent.
func Loaded() bool {
	return exec.Command("/bin/launchctl", "print", service()).Run() == nil
}

// Load bootstraps the agent, which runs the launcher once immediately.
func Load() error {
	if Loaded() {
		Unload()
	}
	out, err := exec.Command("/bin/launchctl", "bootstrap", domain(), state.PlistFile()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Unload boots the agent out of launchd if it is loaded.
func Unload() error {
	if !Loaded() {
		return nil
	}
	out, err := exec.Command("/bin/launchctl", "bootout", service()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl bootout: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
