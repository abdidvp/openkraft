package main

import (
	"os"

	"github.com/openkraft/openkraft/internal/adapters/inbound/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
