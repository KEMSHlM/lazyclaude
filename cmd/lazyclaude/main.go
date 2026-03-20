package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}