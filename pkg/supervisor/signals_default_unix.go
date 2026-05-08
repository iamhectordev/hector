//go:build unix

package supervisor

import (
	"os"
	"syscall"
)

var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
