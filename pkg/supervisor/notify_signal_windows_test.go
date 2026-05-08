//go:build windows

package supervisor_test

import "os"

func notifyTestSignal() os.Signal {
	return os.Interrupt
}

func sendNotifyTestSignal() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(os.Interrupt)
}
