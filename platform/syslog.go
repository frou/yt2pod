// +build !windows,!darwin

package platform

import (
	"io"
	"log/syslog"
)

func NewSyslog(senderName string) (io.Writer, error) {
	return syslog.New(syslog.LOG_DAEMON|syslog.LOG_INFO, senderName)
}
