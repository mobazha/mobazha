package cli

import (
	"os"
	"testing"
)

type recordingStart struct {
	called bool
}

func (command *recordingStart) Execute([]string) error {
	command.called = true
	return nil
}

func TestRunInjectsDistributionStartCommand(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"mobazha", "start"}

	command := new(recordingStart)
	if err := Run(command); err != nil {
		t.Fatal(err)
	}
	if !command.called {
		t.Fatal("injected start command was not executed")
	}
}

func TestRunRejectsMissingStartCommand(t *testing.T) {
	if err := Run(nil); err == nil {
		t.Fatal("expected a missing start command to fail")
	}
}

func TestRunTreatsHelpAsSuccess(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"mobazha", "start", "--help"}

	if err := Run(new(recordingStart)); err != nil {
		t.Fatalf("help should be a successful terminal action: %v", err)
	}
}
