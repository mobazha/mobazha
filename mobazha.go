package main

import (
	"os"

	"github.com/mobazha/mobazha3.0/cmd"
	"github.com/mobazha/mobazha3.0/pkg/cli"
)

func main() {
	if err := cli.Run(&cmd.Start{}); err != nil {
		os.Exit(1)
	}
}
