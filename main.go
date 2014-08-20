package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var help bool
	flag.BoolVar(&help, "help", false, "Show this message")

	flag.Parse()

	if help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}
}
