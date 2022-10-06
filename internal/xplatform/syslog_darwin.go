package xplatform

import (
	"io"
	"log/syslog"
)

func NewSyslog(senderName string) (io.Writer, error) {
	// macOS by default drops messages with LOG_INFO and LOG_DEBUG severity (see the
	// configuration in /etc/asl.conf), so use the least severe that isn't dropped.
	return syslog.New(syslog.LOG_DAEMON|syslog.LOG_NOTICE, senderName)
}
