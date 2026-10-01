// Command tomfoolery is one binary answering to four names: tomfoolery,
// ls-gossip, kill-with-eulogy and uptime-brag. It dispatches on argv[0].
package main

import (
	"os"

	"github.com/EmberGuild-Labs/tomfoolery/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args))
}
