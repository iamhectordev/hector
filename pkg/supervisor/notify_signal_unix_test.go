//go:build unix

package supervisor_test

import (
	"os"
	"syscall"
)

func notifyTestSignal() os.Signal {
	return syscall.SIGUSR1
}

func sendNotifyTestSignal() error {
	return syscall.Kill(os.Getpid(), syscall.SIGUSR1)
}
