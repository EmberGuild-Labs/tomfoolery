package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlockRoundTrip(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	orig := "export FOO=1\nalias ll='ls -l'"
	os.WriteFile(rc, []byte(orig), 0o600)
	if added, err := AddBlock(rc); !added || err != nil {
		t.Fatal(added, err)
	}
	if added, _ := AddBlock(rc); added {
		t.Fatal("block added twice")
	}
	if !HasBlock(rc) {
		t.Fatal("HasBlock")
	}
	if info, _ := os.Stat(rc); info.Mode().Perm() != 0o600 {
		t.Errorf("permissions changed to %v", info.Mode().Perm())
	}
	if removed, err := RemoveBlock(rc); !removed || err != nil {
		t.Fatal(removed, err)
	}
	b, _ := os.ReadFile(rc)
	if string(b) != orig+"\n" {
		t.Errorf("after remove: %q", b)
	}
}

func TestRender(t *testing.T) {
	h := Render("/opt/it's here/tomfoolery", []string{"kill", "pkill"})
	for _, want := range []string{
		`_tomfoolery_bin='/opt/it'\''s here/tomfoolery'`,
		"function ls {", "function kill {", "builtin kill \"$@\"",
		"function pkill {", "command pkill \"$@\"", "RANDOM % 100",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("hook missing %q", want)
		}
	}
	if strings.Contains(h, "function killall") || strings.Contains(h, "%!") {
		t.Error("unexpected content in hook")
	}
}
