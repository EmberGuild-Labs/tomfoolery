// Package intro is the EmberGuild Labs studio intro, ported from eglabs.py
// (the "EGLabs Loading" project) so tomfoolery can play it during install
// with no Python dependency. Drifting ash spirals into the EMBERGUILD logo
// and ignites, a glint sweeps across it, and the letters crumble away on the
// wind. Any key skips it.
package intro

import (
	"fmt"
	"math"
	"os"
	"strings"
)

type rgb [3]int

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func mix(c1, c2 rgb, t float64) rgb {
	t = clamp(t)
	return rgb{
		int(float64(c1[0]) + float64(c2[0]-c1[0])*t),
		int(float64(c1[1]) + float64(c2[1]-c1[1])*t),
		int(float64(c1[2]) + float64(c2[2]-c1[2])*t),
	}
}

func easeInOut(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

var (
	bgColor   = rgb{13, 10, 10} // charcoal
	textColor = rgb{210, 190, 174}
	shine     = rgb{255, 255, 250}
	ashGrey   = rgb{95, 88, 84}
)

// Ember palette: charcoal → deep red → crimson → orange → gold → white-hot.
var heatStops = []struct {
	t float64
	c rgb
}{
	{0.00, rgb{20, 14, 13}},
	{0.18, rgb{72, 12, 8}},
	{0.35, rgb{152, 24, 10}},
	{0.55, rgb{232, 84, 18}},
	{0.72, rgb{255, 150, 40}},
	{0.86, rgb{255, 208, 92}},
	{1.00, rgb{255, 248, 226}},
}

var heatLUT = func() [256]rgb {
	var lut [256]rgb
	for i := range lut {
		t := float64(i) / 255
		lut[i] = heatStops[len(heatStops)-1].c
		for k := 0; k+1 < len(heatStops); k++ {
			a, b := heatStops[k], heatStops[k+1]
			if t <= b.t {
				lut[i] = mix(a.c, b.c, (t-a.t)/(b.t-a.t))
				break
			}
		}
	}
	return lut
}()

func heat(t float64) rgb { return heatLUT[int(clamp(t)*255)] }

var trueColor = func() bool {
	v := strings.ToLower(os.Getenv("EGLABS_COLOR"))
	return v != "256" && v != "8bit"
}()

func rgbTo256(c rgb) int {
	q := func(v int) int {
		switch {
		case v < 48:
			return 0
		case v < 115:
			return 1
		default:
			return (v - 35) / 40
		}
	}
	levels := [6]int{0, 95, 135, 175, 215, 255}
	qr, qg, qb := q(c[0]), q(c[1]), q(c[2])
	cube := rgb{levels[qr], levels[qg], levels[qb]}
	avg := (c[0] + c[1] + c[2]) / 3
	gi := (avg - 8) / 10
	if avg < 8 {
		gi = 0
	} else if avg > 238 {
		gi = 23
	}
	gray := 8 + gi*10
	sq := func(v int) int { return v * v }
	dCube := sq(c[0]-cube[0]) + sq(c[1]-cube[1]) + sq(c[2]-cube[2])
	dGray := sq(c[0]-gray) + sq(c[1]-gray) + sq(c[2]-gray)
	if dGray < dCube {
		return 232 + gi
	}
	return 16 + 36*qr + 6*qg + qb
}

type seqKey struct {
	c     rgb
	layer int
}

var seqCache = map[seqKey]string{}

func colorSeq(c rgb, layer int) string {
	k := seqKey{c, layer}
	if s, ok := seqCache[k]; ok {
		return s
	}
	var s string
	if trueColor {
		s = fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", layer, c[0], c[1], c[2])
	} else {
		s = fmt.Sprintf("\x1b[%d;5;%dm", layer, rgbTo256(c))
	}
	seqCache[k] = s
	return s
}

type cell struct {
	ch     rune
	fg, bg rgb
}

// canvas is a grid of cells: character, foreground and background color.
type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas {
	cv := &canvas{w: w, h: h, cells: make([]cell, w*h)}
	for i := range cv.cells {
		cv.cells[i] = cell{' ', textColor, bgColor}
	}
	return cv
}

func (cv *canvas) put(x, y float64, c rune, fg rgb) {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	if ix >= 0 && ix < cv.w && iy >= 0 && iy < cv.h {
		i := iy*cv.w + ix
		cv.cells[i].ch = c
		cv.cells[i].fg = fg
	}
}

func (cv *canvas) text(x, y int, s string, fg rgb) {
	k := 0
	for _, c := range s {
		cv.put(float64(x+k), float64(y), c, fg)
		k++
	}
}

func (cv *canvas) tint(x, y int, color rgb, a float64) {
	if x >= 0 && x < cv.w && y >= 0 && y < cv.h {
		i := y*cv.w + x
		cv.cells[i].bg = mix(cv.cells[i].bg, color, a)
	}
}
