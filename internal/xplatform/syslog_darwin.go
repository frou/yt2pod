package xplatform

import (
	"io"
	"log/syslog"
)

func NewSyslog(senderName string) (io.Writer, error) {
	// macOS by default swallows LOG_INFO (and LOG_DEBUG) messages, so use the
	// next highest severity.
	return syslog.New(syslog.LOG_DAEMON|syslog.LOG_NOTICE, senderName)
}
