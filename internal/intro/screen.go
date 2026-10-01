package intro

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// screen owns the terminal while the intro plays: alternate screen, hidden
// cursor, cbreak keys, and diffed frame output.
type screen struct {
	in, out *os.File
	saved   *syscall.Termios
	prev    []cell
	w, h    int
}

func ioctl(fd uintptr, req uint, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

func (s *screen) enter() {
	var t syscall.Termios
	if ioctl(s.in.Fd(), syscall.TIOCGETA, unsafe.Pointer(&t)) == nil {
		saved := t
		s.saved = &saved
		t.Lflag &^= syscall.ICANON | syscall.ECHO // cbreak: keys arrive at once, unechoed
		t.Cc[syscall.VMIN] = 1
		t.Cc[syscall.VTIME] = 0
		ioctl(s.in.Fd(), syscall.TIOCSETA, unsafe.Pointer(&t))
	}
	s.write("\x1b[?1049h\x1b[?25l\x1b[?7l\x1b[2J")
}

func (s *screen) exit() {
	s.write("\x1b[0m\x1b[?7h\x1b[?25h\x1b[?1049l")
	if s.saved != nil {
		ioctl(s.in.Fd(), syscall.TIOCSETA, unsafe.Pointer(s.saved))
	}
}

func (s *screen) write(str string) { s.out.WriteString(str) }

// key returns pending input without blocking, or "".
func (s *screen) key() string {
	fd := int(s.in.Fd())
	var set syscall.FdSet
	set.Bits[fd/32] |= 1 << (uint(fd) % 32)
	tv := syscall.Timeval{}
	if err := syscall.Select(fd+1, &set, nil, nil, &tv); err != nil {
		return ""
	}
	if set.Bits[fd/32]&(1<<(uint(fd)%32)) == 0 {
		return ""
	}
	buf := make([]byte, 64)
	n, _ := s.in.Read(buf)
	return string(buf[:n])
}

func (s *screen) size() (int, int) {
	var ws struct{ Row, Col, X, Y uint16 }
	if ioctl(s.out.Fd(), syscall.TIOCGWINSZ, unsafe.Pointer(&ws)) != nil || ws.Col == 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}

func (s *screen) canvas() *canvas {
	w, h := s.size()
	if w != s.w || h != s.h {
		s.w, s.h = w, h
		s.prev = nil
		s.write("\x1b[0m\x1b[2J")
	}
	return newCanvas(w, h)
}

func (s *screen) flush(cv *canvas) {
	var b strings.Builder
	b.WriteString("\x1b[?2026h") // synchronized update: the terminal paints the frame at once
	cur := -1
	var lastFg, lastBg rgb
	haveFg, haveBg := false, false
	for i, c := range cv.cells {
		if s.prev != nil && s.prev[i] == c {
			continue
		}
		if cur != i {
			fmt.Fprintf(&b, "\x1b[%d;%dH", i/cv.w+1, i%cv.w+1)
		}
		if !haveFg || c.fg != lastFg {
			b.WriteString(colorSeq(c.fg, 38))
			lastFg, haveFg = c.fg, true
		}
		if !haveBg || c.bg != lastBg {
			b.WriteString(colorSeq(c.bg, 48))
			lastBg, haveBg = c.bg, true
		}
		b.WriteRune(c.ch)
		if (i+1)%cv.w != 0 {
			cur = i + 1
		} else {
			cur = -1
		}
	}
	b.WriteString("\x1b[?2026l")
	s.write(b.String())
	s.prev = cv.cells
}
