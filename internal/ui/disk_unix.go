//go:build darwin || linux

package ui

import "golang.org/x/sys/unix"

// diskTotal returns the total bytes on the filesystem containing path,
// or 0 if the syscall fails. Used to show a progress percentage during scan.
func diskTotal(path string) int64 {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0
	}
	//nolint:gosec // block size comes from the kernel, not user input
	return int64(st.Blocks) * int64(st.Bsize)
}
