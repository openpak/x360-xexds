// Command x360-xexds-probe runs the dashboard's XEXDS connection probe against
// a host:port and prints pass or fail, so an endpoint can be checked without a
// console or emulator. Exit status 0 means every step passed.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"openpak/x360-xexds/internal/xexds"
)

func main() {
	wait := flag.Duration("timeout", 2*time.Second, "wait for each reply")
	tries := flag.Int("tries", 3, "sends per request before giving up")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: x360-xexds-probe [-timeout 2s] [-tries 3] host:port")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if !xexds.Probe(flag.Arg(0), *wait, *tries, os.Stdout) {
		os.Exit(1)
	}
}
