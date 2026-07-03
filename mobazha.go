package main

import (
	"os"

	"github.com/mobazha/mobazha/cmd"
	"github.com/mobazha/mobazha/pkg/cli"
)

func main() {
	if err := cli.Run(&cmd.Start{}); err != nil {
		os.Exit(1)
	}
}
