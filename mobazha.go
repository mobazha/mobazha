package main

import (
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mobazha/mobazha3.0/cmd"
)

func main() {
	parser := flags.NewParser(nil, flags.Default)

	_, err := parser.AddCommand("status",
		"get the Mobazha node status",
		"The status command gets the Mobazha node status",
		&cmd.Status{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = parser.AddCommand("start",
		"start the Mobazha node",
		"The start command starts the Mobazha node",
		&cmd.Start{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("init",
		"initialize an Mobazha node",
		"The init command creates and initializes a new data directory and database.",
		&cmd.Init{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("doctor",
		"diagnose the Mobazha node environment",
		"The doctor command checks system resources, network connectivity, DNS resolution, Docker status, and node health.",
		&cmd.Doctor{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("backup",
		"back up the Mobazha data directory",
		"The backup command creates a compressed archive of the data directory for safekeeping.",
		&cmd.Backup{})
	if err != nil {
		log.Fatal(err)
	}
	_, err = parser.AddCommand("devnet",
		"start a local dev net",
		"The devnet command spins up a local network of three nodes (buyer, vendor, moderator)"+
			"that connects all three nodes together and uses a mock wallet and mock coins. Each node is pre-populated"+
			"with data for ease of use.",
		&cmd.DevNet{})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
