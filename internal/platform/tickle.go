// +build !windows

package platform

import (
	"syscall"

	"github.com/frou/stdext"
)

func RegisterStalenessResetter(f func()) {
	stdext.HandleSignal(syscall.SIGUSR1, true, f)
}
