//go:build !darwin

package scanner

// GetPurgeableSpace returns the amount of purgeable space in bytes for the volume containing path.
// On non-darwin platforms, this always returns 0 since purgeable space is a macOS-specific concept.
func GetPurgeableSpace(_ string) int64 {
	return 0
}
