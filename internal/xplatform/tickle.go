// +build !windows

package xplatform

import (
	"syscall"

	"github.com/frou/stdext"
)

func RegisterStalenessResetter(f func()) {
	stdext.HandleSignal(syscall.SIGUSR1, true, f)
}
