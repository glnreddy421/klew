package main

import (
	"os"

	"github.com/glnreddy421/klew/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
