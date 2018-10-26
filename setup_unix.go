// +build !windows

package main

import (
	"log"
	"syscall"
	"time"

	"github.com/frou/stdext"
)

func registerTickle() {
	stdext.HandleSignal(syscall.SIGUSR1, true, func() {
		lastTimeAnyFeedWritten.Set(time.Now())
		log.Print("Reset the clock for stale feeds, due to signal")
	})
}
