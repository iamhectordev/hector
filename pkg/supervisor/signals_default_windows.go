//go:build windows

package supervisor

import "os"

var defaultSignals = []os.Signal{os.Interrupt}
