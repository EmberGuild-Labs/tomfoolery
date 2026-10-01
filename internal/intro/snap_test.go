package intro

import (
	"fmt"
	"html"
	"os"
	"strings"
	"testing"
)

// TestSnapshot writes HTML frames for visual comparison when INTRO_SNAP is set.
func TestSnapshot(t *testing.T) {
	out := os.Getenv("INTRO_SNAP")
	if out == "" {
		t.Skip()
	}
	seq := newSequence()
	var b strings.Builder
	want := map[int]bool{60: true, 135: true, 168: true, 240: true}
	for f := 0; f <= 240; f++ {
		cv := newCanvas(100, 26)
		seq.frame(cv, float64(f)/fps, 1.0/fps)
		if !want[f] {
			continue
		}
		fmt.Fprintf(&b, "<h3>Go t=%.1fs</h3><pre>", float64(f)/fps)
		for y := 0; y < cv.h; y++ {
			for x := 0; x < cv.w; x++ {
				c := cv.cells[y*cv.w+x]
				fmt.Fprintf(&b, `<span style="color:rgb(%d,%d,%d);background:rgb(%d,%d,%d)">%s</span>`,
					c.fg[0], c.fg[1], c.fg[2], c.bg[0], c.bg[1], c.bg[2], html.EscapeString(string(c.ch)))
			}
			b.WriteString("\n")
		}
		b.WriteString("</pre>")
	}
	os.WriteFile(out, []byte(b.String()), 0o644)
}
