package intro

import (
	"strings"
	"testing"
)

// Plays the whole sequence headlessly and checks the key beats: the logo is
// fully lit after ignition, and the screen is empty (black) at the end.
func TestSequence(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {160, 45}, {64, 14}} {
		seq := newSequence()
		dt := 1.0 / fps
		var litAt45, finishedAt float64
		for f := 0; ; f++ {
			tm := float64(f) * dt
			cv := newCanvas(size[0], size[1])
			done := seq.frame(cv, tm, dt)
			if f == int(4.5*fps) {
				n := 0
				for _, c := range seq.L.cells {
					if cv.cells[c.y*cv.w+c.x].ch == '█' {
						n++
					}
				}
				litAt45 = float64(n) / float64(len(seq.L.cells))
			}
			if done {
				finishedAt = tm
				for _, c := range cv.cells {
					if c.ch != ' ' {
						t.Errorf("%v: screen not empty at the end (%q)", size, c.ch)
						break
					}
				}
				break
			}
		}
		if litAt45 < 0.9 { // embers drawn over the letters hide a few cells
			t.Errorf("%v: only %.0f%% of the logo lit at 4.5s", size, litAt45*100)
		}
		if finishedAt < 9 || finishedAt >= maxT {
			t.Errorf("%v: finished at %.1fs, want ~10.5s", size, finishedAt)
		}
	}
}

func TestTagline(t *testing.T) {
	L := newLogo(80, 10)
	var b strings.Builder
	for _, c := range L.tag {
		b.WriteRune(c.c)
	}
	if !strings.Contains(b.String(), "LABS") {
		t.Errorf("tagline %q", b.String())
	}
	if L.width != 57 {
		t.Errorf("logo width %d, want 57 (matches eglabs.py)", L.width)
	}
}
