package cli

import (
	"os"

	"github.com/EmberGuild-Labs/tomfoolery/internal/intro"
)

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// playIntro shows the built-in EmberGuild Labs studio intro. Any key skips
// it, and it never blocks setup: without a terminal it simply doesn't play.
func playIntro() bool {
	if os.Getenv("TOMFOOLERY_NO_INTRO") != "" {
		return false
	}
	return intro.Play()
}
