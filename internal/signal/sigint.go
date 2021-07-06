package signal

import (
	"os"
	"os/signal"
	"syscall"
)

var ch chan os.Signal

func init() {
	ch = make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

// SIGINT function returns true if ctrl+c was pressed
func SIGINT() bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
