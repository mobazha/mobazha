//go:build !desktop

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mobazha/mobazha3.0/internal/supervisor"
)

type headlessUI struct {
	logger *log.Logger
}

func createUI(logger *log.Logger) supervisor.LauncherUI {
	return &headlessUI{logger: logger}
}

func (h *headlessUI) Run(s *supervisor.Supervisor) {
	h.logger.Println("Headless mode — waiting for signals (SIGINT/SIGTERM)")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	h.logger.Printf("Received %s, shutting down...", sig)
	s.Stop()
}

func (h *headlessUI) OnStatusChange(status supervisor.Status) {
	h.logger.Printf("Node status: %s", status)
}
