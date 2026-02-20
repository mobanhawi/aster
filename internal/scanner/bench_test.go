package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mobanhawi/aster/internal/scanner"
)

// buildFlatTree creates a directory with n files of the given size.
func buildFlatTree(b *testing.B, n int, fileSize int) string {
	b.Helper()
	root := b.TempDir()
	for i := range n {
		path := filepath.Join(root, "file_"+strconv.Itoa(i)+".bin")
		if err := os.WriteFile(path, make([]byte, fileSize), 0o644); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
	return root
}

// buildDeepTree creates a deeply nested directory tree: depth levels, each with
// breadth subdirs and filesPerDir files.
func buildDeepTree(b *testing.B, depth, breadth, filesPerDir, fileSize int) string {
	b.Helper()
	root := b.TempDir()
	var fill func(dir string, d int)
	fill = func(dir string, d int) {
		for i := range filesPerDir {
			path := filepath.Join(dir, "f"+strconv.Itoa(i)+".bin")
			if err := os.WriteFile(path, make([]byte, fileSize), 0o644); err != nil {
				b.Fatalf("write: %v", err)
			}
		}
		if d <= 0 {
			return
		}
		for i := range breadth {
			sub := filepath.Join(dir, "d"+strconv.Itoa(i))
			if err := os.Mkdir(sub, 0o755); err != nil {
				b.Fatalf("mkdir: %v", err)
			}
			fill(sub, d-1)
		}
	}
	fill(root, depth)
	return root
}

// BenchmarkScanFlat measures scanning a single directory with many files —
// typical of Downloads or node_modules.
func BenchmarkScanFlat(b *testing.B) {
	for _, n := range []int{100, 1_000, 10_000} {
		root := buildFlatTree(b, n, 0)
		b.Run(strconv.Itoa(n)+"_files", func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				_, err := scanner.Scan(context.Background(), root, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkScanDeep measures scanning a wide, deep tree — typical of a
// project with many nested source directories.
func BenchmarkScanDeep(b *testing.B) {
	// depth=4, breadth=4, 10 files/dir → 4^0+4^1+4^2+4^3+4^4 = 341 dirs, ~3 410 files
	root := buildDeepTree(b, 4, 4, 10, 0)
	b.ResetTimer()
	for range b.N {
		_, err := scanner.Scan(context.Background(), root, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScanWithProgress measures the overhead of the progress channel.
func BenchmarkScanWithProgress(b *testing.B) {
	root := buildFlatTree(b, 1_000, 128)
	b.ResetTimer()
	for range b.N {
		ch := make(chan int64, 4096)
		go func() {
			for range ch { //nolint:revive
			}
		}()
		_, err := scanner.Scan(context.Background(), root, ch)
		close(ch)
		if err != nil {
			b.Fatal(err)
		}
	}
}
