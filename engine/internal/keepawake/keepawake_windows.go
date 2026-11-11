//go:build windows

package keepawake

import "syscall"

const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	setThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
)

// Enable pide a Windows que no suspenda el sistema mientras el proceso vive.
func Enable() {
	setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired))
}

// Disable libera la petición de keep-awake.
func Disable() {
	setThreadExecutionState.Call(uintptr(esContinuous))
}
