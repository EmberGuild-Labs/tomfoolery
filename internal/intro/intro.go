package intro

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	fps      = 30
	minW     = 64
	minH     = 14
	travel   = 1.5  // seconds each speck of ash takes to spiral into place
	ignite   = 3.2  // centre letters catch fire; it spreads outward over ~0.8s
	glintAt  = 5.3  // a shine sweeps across the finished logo
	glintLen = 0.9  //
	crumble  = 7.5  // letters break up left to right and blow away
	black    = 0.35 // seconds of black before handing the terminal back
	maxT     = 14.0 // safety net
)

var (
	ashGlyphs      = []rune("·.:',`")
	scramble       = []rune(":;+x*%")
	fragmentGlyphs = []rune("▓▒░")
)

func pickRune(rs []rune) rune { return rs[rand.Intn(len(rs))] }

func uniform(a, b float64) float64 { return a + (b-a)*rand.Float64() }

type flyer struct {
	sx, sy          float64
	tx, ty          int
	delay, ph1, ph2 float64
	glyph           rune
	ignite          float64
	shade           int
}

type loose struct {
	x, y, ph float64
	glyph    rune
	shade    int
	ignite   float64
}

type crumbleParams struct {
	ct, life, ph, rise float64
	glyph              rune
}

type sequence struct {
	parts      particles
	w, h       int
	L          *logo
	cx, cy     float64
	flyers     []flyer
	loose      []loose
	crumble    []crumbleParams
	tagCrumble []crumbleParams
	crumbleEnd float64
	lit        map[int]bool
	endAt      float64
}

func newSequence() *sequence { return &sequence{lit: map[int]bool{}, endAt: -1} }

// logoTop is the row that vertically centres the logo + tagline (7 rows).
func logoTop(h int) int {
	if t := floorDiv(h-7, 2); t > 2 {
		return t
	}
	return 2
}

func (s *sequence) layout(cv *canvas) *logo {
	if s.w == cv.w && s.h == cv.h && s.L != nil {
		return s.L
	}
	s.w, s.h = cv.w, cv.h
	L := newLogo(cv.w, logoTop(cv.h))
	s.L = L
	s.cx, s.cy = L.cx, float64(L.top+2)
	half := float64(L.width) / 2
	W, H := float64(cv.w), float64(cv.h)
	s.flyers = s.flyers[:0]
	for _, c := range L.cells {
		x := float64(c.x)
		s.flyers = append(s.flyers, flyer{
			sx: uniform(1, W-2), sy: uniform(2, H-3), tx: c.x, ty: c.y,
			delay:  0.3 + (x-float64(L.x0))/float64(L.width)*0.8 + uniform(0, 0.4),
			ph1:    uniform(0, 6.3),
			ph2:    uniform(0, 6.3),
			glyph:  pickRune(ashGlyphs),
			ignite: ignite + math.Abs(x-L.cx)/half*0.8 + uniform(0, 0.12),
			shade:  70 + rand.Intn(56),
		})
	}
	s.loose = s.loose[:0]
	n := cv.w * cv.h / 40
	if n < 40 {
		n = 40
	}
	for i := 0; i < n; i++ {
		x := uniform(1, W-2)
		s.loose = append(s.loose, loose{
			x: x, y: uniform(2, H-3), ph: uniform(0, 6.3), glyph: pickRune(ashGlyphs),
			shade:  55 + rand.Intn(46),
			ignite: ignite + math.Min(1, math.Abs(x-L.cx)/half)*0.8 + uniform(0, 0.4),
		})
	}
	// per-cell crumble: when it breaks off, how long the fragment lives, how it drifts
	s.crumble = s.crumble[:0]
	for _, c := range L.cells {
		s.crumble = append(s.crumble, s.crumbleFor(c.x))
	}
	s.tagCrumble = s.tagCrumble[:0]
	for _, c := range L.tag {
		s.tagCrumble = append(s.tagCrumble, s.crumbleFor(c.x))
	}
	s.crumbleEnd = 0
	for _, c := range append(append([]crumbleParams{}, s.crumble...), s.tagCrumble...) {
		s.crumbleEnd = math.Max(s.crumbleEnd, c.ct+c.life)
	}
	return L
}

func (s *sequence) crumbleFor(x int) crumbleParams {
	L := s.L
	return crumbleParams{
		ct:    crumble + float64(x-L.x0)/float64(L.width)*1.1 + uniform(0, 0.3),
		life:  uniform(0.8, 1.3),
		ph:    uniform(0, 6.3),
		rise:  uniform(0.5, 2.5),
		glyph: pickRune(ashGlyphs),
	}
}

// glint is the strength of the diagonal shine at a cell, 0..1.
func (s *sequence) glint(t float64, x, y int) float64 {
	u := (t - glintAt) / glintLen
	if u < 0 || u > 1 {
		return 0
	}
	L := s.L
	sx := float64(L.sx)
	pos := float64(L.x0) - 12*sx + (float64(L.width)+24*sx)*easeInOut(u)
	d := float64(x) - pos + float64(y-L.top-2)*1.3*sx // leans like "/"
	return math.Exp(-math.Pow(d/(1.8*sx), 2))
}

// fragment draws a piece that broke off at time ct, carried right by the
// wind while it cools to ash.
func (s *sequence) fragment(cv *canvas, t float64, x, y int, p crumbleParams, first rune, colHot float64) {
	a := t - p.ct
	k := a / p.life
	if k >= 1 {
		return
	}
	X := float64(x) + 5*a + 16*a*a + 0.6*math.Sin(a*6+p.ph)
	Y := float64(y) - p.rise*a + 0.4*math.Sin(a*5+p.ph)
	c := p.glyph
	switch {
	case k < 0.15:
		c = first
	case k < 0.35:
		c = fragmentGlyphs[1]
	case k < 0.6:
		c = fragmentGlyphs[2]
	}
	cv.put(X, Y, c, mix(heat(colHot-0.55*k), ashGrey, (k-0.3)/0.6))
}

type flying struct {
	x, y float64
	c    rune
	col  rgb
}

// frame draws one frame and reports whether the intro has finished.
func (s *sequence) frame(cv *canvas, t, dt float64) bool {
	L := s.layout(cv)
	cx, cy := s.cx, s.cy
	const swirl = 2.4

	// loose ash drifts, then flares up and burns away as the ignition passes
	for _, l := range s.loose {
		x := l.x + 1.6*math.Sin(t*0.7+l.ph) + t*0.6
		y := l.y + 0.6*math.Sin(t*0.9+l.ph*1.3)
		if t < l.ignite {
			cv.put(x, y, l.glyph, rgb{l.shade, l.shade - 6, l.shade - 10})
		} else if a := t - l.ignite; a < 1.1 {
			c := '·'
			if a < 0.3 {
				c = '*'
			}
			cv.put(x, y-a*a*4, c, heat(0.95-a*0.7))
		}
	}

	heats := make([]float64, len(L.cells))
	colors := make([]*rgb, len(L.cells))
	var fly []flying
	for j, f := range s.flyers {
		e := clamp((t - f.delay) / travel)
		if e >= 1 {
			cp := s.crumble[j]
			if t >= cp.ct {
				s.fragment(cv, t, f.tx, f.ty, cp, '▓', 0.9)
				continue
			}
			if t >= f.ignite {
				h := 0.6 + 0.4*math.Exp(-(t-f.ignite)*2.5) + flicker(t, f.tx, f.ty)
				if cp.ct-t < 0.35 { // the burning edge just before it breaks off
					h += 0.45 * (1 - (cp.ct-t)/0.35)
				}
				gl := s.glint(t, f.tx, f.ty)
				heats[j] = h + 0.4*gl
				col := mix(heat(h), shine, 0.85*gl)
				colors[j] = &col
				if !s.lit[j] {
					s.lit[j] = true
					if _, above := L.index[pt{f.tx, f.ty - 1}]; !above && rand.Float64() < 0.5 {
						s.parts.emit(float64(f.tx)+rand.Float64(), float64(f.ty)-0.2,
							uniform(-1.5, 1.5), -uniform(3, 7), uniform(0.6, 1.6), 1.0)
					}
				}
			} else {
				heats[j] = -1 // smouldering
			}
			continue
		}
		px := f.sx + 1.8*math.Sin(t*0.8+f.ph1)
		py := f.sy + 0.7*math.Sin(t*0.6+f.ph2)
		e = easeInOut(e)
		// spiral in: interpolate in aspect-corrected space, rotating as we close in
		ax, ay := (px-cx)*0.5, py-cy
		ca, sa := math.Cos(-swirl), math.Sin(-swirl)
		ax, ay = ax*ca-ay*sa, ax*sa+ay*ca
		bx, by := (float64(f.tx)-cx)*0.5, float64(f.ty)-cy
		lx, ly := lerp(ax, bx, e), lerp(ay, by, e)
		r := swirl * (1 - e)
		ca, sa = math.Cos(r), math.Sin(r)
		X := cx + (lx*ca-ly*sa)*2
		Y := cy + (lx*sa + ly*ca)
		ash := rgb{f.shade, f.shade - 6, f.shade - 10}
		switch {
		case e < 0.5:
			fly = append(fly, flying{X, Y, f.glyph, ash})
		case e < 0.92:
			fly = append(fly, flying{X, Y, pickRune(scramble), mix(ash, rgb{160, 44, 16}, (e-0.5)/0.42)})
		default:
			fly = append(fly, flying{X, Y, '▒', rgb{116, 32, 12}})
		}
	}

	L.glow(cv, heats)
	for i, c := range L.cells {
		if colors[i] != nil {
			cv.put(float64(c.x), float64(c.y), '█', *colors[i])
		} else if heats[i] < 0 {
			cv.put(float64(c.x), float64(c.y), '▓',
				mix(rgb{58, 15, 9}, rgb{104, 26, 11}, 0.5+0.5*math.Sin(t*4+float64(c.x)*0.7+float64(c.y))))
		}
	}
	for _, f := range fly {
		cv.put(f.x, f.y, f.c, f.col)
	}

	for i, c := range L.tag {
		cp := s.tagCrumble[i]
		appear := ignite + 0.7 + math.Abs(float64(c.x)-L.cx)/(float64(L.width)/2)*0.6
		if t < appear {
			continue
		}
		if t >= cp.ct {
			s.fragment(cv, t, c.x, c.y, cp, c.c, 0.7)
			continue
		}
		col := heat(0.55 + 0.4*math.Exp(-(t-appear)*3))
		if c.c == '─' {
			col = mix(bgColor, col, 0.55)
		}
		cv.put(float64(c.x), float64(c.y), c.c, mix(col, shine, 0.85*s.glint(t, c.x, c.y)))
	}

	// embers off the lit letters, until the logo starts to crumble
	if t >= ignite+0.8 && t < crumble+1.4 {
		const rate = 45.0
		n := int(rate * dt)
		if rand.Float64() < math.Mod(rate*dt, 1) {
			n++
		}
		for k := 0; k < n; k++ {
			j := rand.Intn(len(L.cells))
			c := L.cells[j]
			if _, above := L.index[pt{c.x, c.y - 1}]; heats[j] > 0.5 && !above {
				s.parts.emit(float64(c.x)+rand.Float64(), float64(c.y)-0.2,
					uniform(-1.2, 1.2), -uniform(2.5, 6), uniform(0.7, 1.8), 0.9)
			}
		}
	}
	s.parts.update(dt, t)
	s.parts.draw(cv, t)

	// done once the last fragment has blown away and the embers have died out
	if s.endAt < 0 && t >= s.crumbleEnd && len(s.parts.p) == 0 {
		s.endAt = t + black
	}
	return (s.endAt >= 0 && t >= s.endAt) || t >= maxT
}

// Play runs the intro on the terminal. Any key (or Ctrl-C) skips it. It
// returns false without drawing anything if stdin/stdout are not terminals.
func Play() bool {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
		return false
	}
	scr := &screen{in: os.Stdin, out: os.Stdout}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)
	scr.enter()
	defer scr.exit()

	seq := newSequence()
	frameDt := time.Second / fps
	start := time.Now()
	last := start
	for {
		select {
		case <-sigs:
			return true
		default:
		}
		if scr.key() != "" {
			return true
		}
		now := time.Now()
		t := now.Sub(start).Seconds()
		dt := math.Min(now.Sub(last).Seconds(), 0.1)
		last = now
		cv := scr.canvas()
		var finished bool
		if cv.w < minW || cv.h < minH {
			msg := fmt.Sprintf("Make the terminal at least %d×%d (%d×%d now)", minW, minH, cv.w, cv.h)
			x := (cv.w - len([]rune(msg))) / 2
			if x < 0 {
				x = 0
			}
			cv.text(x, cv.h/2, msg, textColor)
			finished = t >= maxT
		} else {
			finished = seq.frame(cv, t, dt)
		}
		scr.flush(cv)
		if finished {
			return true
		}
		if d := frameDt - time.Since(now); d > 0 {
			time.Sleep(d)
		}
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
