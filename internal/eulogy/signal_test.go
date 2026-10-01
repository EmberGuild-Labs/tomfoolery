package eulogy

import (
	"reflect"
	"testing"
)

func TestParseKill(t *testing.T) {
	cases := []struct {
		args []string
		sig  string
		pids []int
		ok   bool
	}{
		{[]string{"4821"}, "TERM", []int{4821}, true},
		{[]string{"4821", "4822"}, "TERM", []int{4821, 4822}, true},
		{[]string{"-9", "4821"}, "KILL", []int{4821}, true},
		{[]string{"-KILL", "4821"}, "KILL", []int{4821}, true},
		{[]string{"-SIGKILL", "4821"}, "KILL", []int{4821}, true},
		{[]string{"-kill", "4821"}, "KILL", []int{4821}, true},
		{[]string{"-s", "INT", "4821"}, "INT", []int{4821}, true},
		{[]string{"-s", "SIGHUP", "4821"}, "HUP", []int{4821}, true},
		{[]string{"-n", "9", "4821"}, "KILL", []int{4821}, true},
		{[]string{"--", "4821"}, "TERM", []int{4821}, true},
		{[]string{"-TERM", "--", "4821"}, "TERM", []int{4821}, true},
		{[]string{"-6", "4821"}, "ABRT", []int{4821}, true},

		// Everything below must pass straight through.
		{[]string{}, "", nil, false},
		{[]string{"-l"}, "", nil, false},
		{[]string{"-L"}, "", nil, false},
		{[]string{"-l", "9"}, "", nil, false},
		{[]string{"-0", "4821"}, "", nil, false},
		{[]string{"-USR1", "4821"}, "", nil, false},
		{[]string{"-STOP", "4821"}, "", nil, false},
		{[]string{"-CONT", "4821"}, "", nil, false},
		{[]string{"-s", "WINCH", "4821"}, "", nil, false},
		{[]string{"%1"}, "", nil, false},
		{[]string{"-9", "%1"}, "", nil, false},
		{[]string{"4821", "%1"}, "", nil, false},
		{[]string{"--", "-4821"}, "", nil, false},
		{[]string{"-9", "--", "-4821"}, "", nil, false},
		{[]string{"-4821"}, "", nil, false},
		{[]string{"0"}, "", nil, false},
		{[]string{"1"}, "", nil, false},
		{[]string{"Slack"}, "", nil, false},
		{[]string{"-s"}, "", nil, false},
		{[]string{"-n", "KILL", "4821"}, "", nil, false},
		{[]string{"-9"}, "", nil, false},
		{[]string{"-BOGUS", "4821"}, "", nil, false},
		{[]string{"12abc"}, "", nil, false},
	}
	for _, c := range cases {
		p, ok := ParseKill(c.args)
		if ok != c.ok {
			t.Errorf("ParseKill(%q) ok=%v, want %v", c.args, ok, c.ok)
			continue
		}
		if ok && (p.Signal != c.sig || !reflect.DeepEqual(p.PIDs, c.pids)) {
			t.Errorf("ParseKill(%q) = %s %v, want %s %v", c.args, p.Signal, p.PIDs, c.sig, c.pids)
		}
	}
}

func TestSignalName(t *testing.T) {
	for in, want := range map[string]string{
		"9": "KILL", "15": "TERM", "SIGTERM": "TERM", "term": "TERM", "IOT": "ABRT",
		"6": "ABRT", "": "", "99": "", "NOPE": "",
	} {
		if got := SignalName(in); got != want {
			t.Errorf("SignalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePkill(t *testing.T) {
	sig, rest, ok := ParsePkill([]string{"-HUP", "-f", "node server"})
	if !ok || sig != "HUP" || !reflect.DeepEqual(rest, []string{"-f", "node server"}) {
		t.Fatalf("got %s %q %v", sig, rest, ok)
	}
	if _, rest, ok := ParsePkill([]string{"-f", "x"}); !ok || rest[0] != "-f" {
		t.Fatalf("-f must stay a pgrep flag, got %q %v", rest, ok)
	}
	for _, args := range [][]string{{"-USR1", "x"}, {"-I", "x"}, {"-l", "x"}, {}, {"-9"}} {
		if _, _, ok := ParsePkill(args); ok {
			t.Errorf("ParsePkill(%q) should pass through", args)
		}
	}
}

func TestParseKillall(t *testing.T) {
	sig, names, ok := ParseKillall([]string{"-9", "Safari"})
	if !ok || sig != "KILL" || !reflect.DeepEqual(names, []string{"Safari"}) {
		t.Fatalf("got %s %q %v", sig, names, ok)
	}
	for _, args := range [][]string{{"-s", "Safari"}, {"-m", "Saf.*"}, {"Safari", "-v"}, {"-STOP", "x"}, {}} {
		if _, _, ok := ParseKillall(args); ok {
			t.Errorf("ParseKillall(%q) should pass through", args)
		}
	}
}
