package main

import (
	"os"

	"github.com/abdidvp/openkraft/internal/adapters/inbound/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
