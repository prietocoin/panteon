//go:build !windows

package keepawake

// Enable es un no-op fuera de Windows.
func Enable() {}

// Disable es un no-op fuera de Windows.
func Disable() {}
