//go:build !darwin && !linux

package ui

// diskTotal returns 0 on unsupported platforms.
func diskTotal(_ string) int64 { return 0 }
