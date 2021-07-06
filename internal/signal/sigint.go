package signal

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/elastic/elastic-package/internal/logger"
)

var ch chan os.Signal

// Enable function enables signal notifications.
func Enable() {
	ch = make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

// SIGINT function returns true if ctrl+c was pressed
func SIGINT() bool {
	select {
	case <-ch:
		logger.Info("Signal caught!")
		return true
	default:
		return false
	}
}
