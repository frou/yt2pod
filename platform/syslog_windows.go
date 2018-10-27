package platform

import (
	"fmt"
	"io"
)

func NewSyslog(senderName string) (io.Writer, error) {
	return nil, fmt.Errorf("syslog is not supported on Windows")
}
