package intro

import (
	"math"
	"math/rand"
	"strings"
)

const word = "EMBERGUILD"

var font = map[rune][]string{
	'E': {"#####", "#....", "####.", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#"},
	'B': {"####.", "#...#", "####.", "#...#", "####."},
	'R': {"####.", "#...#", "####.", "#..#.", "#...#"},
	'G': {".####", "#....", "#..##", "#...#", ".####"},
	'U': {"#...#", "#...#", "#...#", "#...#", ".###."},
	'I': {"###", ".#.", ".#.", ".#.", "###"},
	'L': {"#....", "#....", "#....", "#....", "#####"},
	'D': {"####.", "#...#", "#...#", "#...#", "####."},
}

type pt struct{ x, y int }

type logoCell struct{ x, y int }

type haloCell struct {
	x, y, j int
	f       float64
}

type tagCell struct {
	x, y int
	c    rune
}

// logo is block-letter EMBERGUILD centred at row top, plus the LABS tagline
// and a glow halo.
type logo struct {
	sx, width, x0, top int
	cx                 float64
	cells              []logoCell
	index              map[pt]int
	halo               []haloCell
	tag                []tagCell
}

func newLogo(w, top int) *logo {
	L := &logo{top: top, sx: 1, index: map[pt]int{}}
	if w >= 122 {
		L.sx = 2 // 2 columns per font pixel keeps letters square
	}
	gap := L.sx
	for i, c := range word {
		L.width += len(font[c][0]) * L.sx
		if i > 0 {
			L.width += gap
		}
	}
	L.x0 = floorDiv(w-L.width, 2)
	L.cx = float64(L.x0) + float64(L.width)/2
	x := L.x0
	for _, c := range word {
		g := font[c]
		gw := len(g[0]) * L.sx
		for ry, row := range g {
			for rx, ch := range row {
				if ch == '#' {
					for k := 0; k < L.sx; k++ {
						L.cells = append(L.cells, logoCell{x + rx*L.sx + k, top + ry})
					}
				}
			}
		}
		x += gw + gap
	}
	for i, c := range L.cells {
		L.index[pt(c)] = i
	}
	L.buildHalo()
	L.buildTagline(top + 6)
	return L
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func (L *logo) buildHalo() {
	const radius = 4.5
	for y := L.top - 2; y < L.top+7; y++ {
		for x := L.x0 - 5; x < L.x0+L.width+5; x++ {
			if _, ok := L.index[pt{x, y}]; ok {
				continue
			}
			best, bd := -1, radius
			for dy := -2; dy <= 2; dy++ {
				for dx := -4; dx <= 4; dx++ {
					if j, ok := L.index[pt{x + dx, y + dy}]; ok {
						d := math.Hypot(float64(dx), float64(dy*2)) // cells are twice as tall as wide
						if d < bd {
							best, bd = j, d
						}
					}
				}
			}
			if best >= 0 {
				f := 1 - bd/radius
				L.halo = append(L.halo, haloCell{x, y, best, f * f})
			}
		}
	}
}

func (L *logo) buildTagline(y int) {
	sep := "   "
	if L.sx != 1 {
		sep = "     "
	}
	core := strings.Join(strings.Split("LABS", ""), sep)
	side := (L.width - len(core) - 4) / 2
	if side < 3 {
		side = 3
	}
	s := []rune(strings.Repeat("─", side) + "  " + core + "  " + strings.Repeat("─", side))
	x0 := int(L.cx - float64(len(s))/2)
	for i, c := range s {
		if c != ' ' {
			L.tag = append(L.tag, tagCell{x0 + i, y, c})
		}
	}
}

func (L *logo) glow(cv *canvas, heats []float64) {
	for _, h := range L.halo {
		v := heats[h.j]
		if v > 0.35 {
			cv.tint(h.x, h.y, heat(v), 0.32*h.f*(v-0.35)/0.65)
		}
	}
}

func flicker(t float64, x, y int) float64 {
	fx, fy := float64(x), float64(y)
	return 0.045 * (math.Sin(t*9.1+fx*1.7+fy*2.3) + math.Sin(t*5.3-fx*0.9))
}

// particle: embers drift upward and twinkle; sparks fly ballistically.
type particle struct {
	x, y, vx, vy, life, maxLife, hot, gravity, phase float64
	ember                                            bool
}

type particles struct{ p []particle }

const maxParticles = 700

func (ps *particles) emit(x, y, vx, vy, life, hot float64) {
	if len(ps.p) < maxParticles {
		ps.p = append(ps.p, particle{x, y, vx, vy, life, life, hot, 0, rand.Float64() * 6.283, true})
	}
}

func (ps *particles) update(dt, t float64) {
	alive := ps.p[:0]
	for _, e := range ps.p {
		e.life -= dt
		if e.life <= 0 {
			continue
		}
		e.vy += e.gravity * dt
		if e.ember {
			e.x += (e.vx + 1.4*math.Sin(t*3.1+e.phase)) * dt
		} else {
			e.x += e.vx * dt
			e.vx *= 1 - 1.5*dt // air drag
		}
		e.y += e.vy * dt
		alive = append(alive, e)
	}
	ps.p = alive
}

func (ps *particles) draw(cv *canvas, t float64) {
	for _, e := range ps.p {
		a := e.life / e.maxLife
		if e.ember {
			fy := e.y - math.Floor(e.y) // pick the glyph that sits at the sub-cell height
			c := '.'
			if fy < 0.33 {
				c = '\''
			} else if fy < 0.66 {
				c = '·'
			}
			twinkle := 0.75 + 0.25*math.Sin(t*18+e.phase*5)
			cv.put(e.x, e.y, c, heat((0.28+0.68*a)*e.hot*twinkle))
			continue
		}
		var c rune
		if a > 0.45 && math.Abs(e.vx)+math.Abs(e.vy) > 12 {
			switch {
			case math.Abs(e.vx) > 2.5*math.Abs(e.vy):
				c = '-'
			case math.Abs(e.vy) > 2.5*math.Abs(e.vx):
				c = '|'
			case e.vx*e.vy < 0:
				c = '/'
			default:
				c = '\\'
			}
		} else {
			switch {
			case a > 0.7:
				c = '*'
			case a > 0.45:
				c = '+'
			case a > 0.2:
				c = '·'
			default:
				c = '.'
			}
		}
		cv.put(e.x, e.y, c, heat(0.3+0.7*a*e.hot))
	}
}
