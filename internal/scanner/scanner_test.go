package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mobanhawi/aster/internal/scanner"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

const (
	fileSizeSmall  = 100
	fileSizeMedium = 500
	fileSizeLarge  = 1000
)

// makeTestDir creates a temporary directory tree and returns its root path.
// layout: map of relpath → content (if empty, it's a directory).
func makeTestDir(t *testing.T, layout map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range layout {
		fullPath := filepath.Join(root, rel)
		if content == nil {
			if err := os.MkdirAll(fullPath, 0o755); err != nil {
				t.Fatalf("makeTestDir: mkdir %s: %v", fullPath, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatalf("makeTestDir: mkdir parent %s: %v", fullPath, err)
			}
			if err := os.WriteFile(fullPath, content, 0o644); err != nil {
				t.Fatalf("makeTestDir: write %s: %v", fullPath, err)
			}
		}
	}
	return root
}

func bytes(n int) []byte { return make([]byte, n) }

// ── Node tests ────────────────────────────────────────────────────────────────

func TestNodeSize(t *testing.T) {
	testCases := []struct {
		name   string
		add    []int64
		wantSz int64
	}{
		{
			name:   "GivenNoAdds_WhenSizeRead_ThenReturnsZero",
			add:    nil,
			wantSz: 0,
		},
		{
			name:   "GivenSingleAdd_WhenSizeRead_ThenReturnsValue",
			add:    []int64{fileSizeLarge},
			wantSz: fileSizeLarge,
		},
		{
			name:   "GivenMultipleAdds_WhenSizeRead_ThenReturnsCumulativeSum",
			add:    []int64{fileSizeMedium, fileSizeMedium, fileSizeSmall},
			wantSz: fileSizeMedium*2 + fileSizeSmall,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n := &scanner.Node{}
			for _, v := range tc.add {
				n.AddSize(v)
			}
			if got := n.Size(); got != tc.wantSz {
				t.Errorf("Size() = %d, want %d", got, tc.wantSz)
			}
		})
	}
}

func TestNodeSortBySize(t *testing.T) {
	t.Run("GivenUnsortedChildren_WhenSortBySize_ThenLargestFirst", func(t *testing.T) {
		parent := &scanner.Node{Name: "parent", IsDir: true}
		small := &scanner.Node{Name: "small"}
		small.SetSize(fileSizeSmall)
		large := &scanner.Node{Name: "large"}
		large.SetSize(fileSizeLarge)
		medium := &scanner.Node{Name: "medium"}
		medium.SetSize(fileSizeMedium)

		parent.Children = []*scanner.Node{small, large, medium}
		parent.SortBySize()

		want := []string{"large", "medium", "small"}
		for i, child := range parent.Children {
			if child.Name != want[i] {
				t.Errorf("children[%d].Name = %q, want %q", i, child.Name, want[i])
			}
		}
	})
}

func TestNodeSortByName(t *testing.T) {
	t.Run("GivenUnsortedChildren_WhenSortByName_ThenAlphabeticalOrder", func(t *testing.T) {
		parent := &scanner.Node{Name: "parent", IsDir: true}
		parent.Children = []*scanner.Node{
			{Name: "zebra"},
			{Name: "apple"},
			{Name: "mango"},
		}
		parent.SortByName()

		want := []string{"apple", "mango", "zebra"}
		for i, child := range parent.Children {
			if child.Name != want[i] {
				t.Errorf("children[%d].Name = %q, want %q", i, child.Name, want[i])
			}
		}
	})
}

// ── Scanner tests ─────────────────────────────────────────────────────────────

func TestScan(t *testing.T) {
	testCases := []struct {
		name        string
		layout      map[string][]byte
		wantMinSize int64
		wantDirs    int // minimum number of dir nodes expected
		wantFiles   int // minimum number of file nodes expected
		wantErr     bool
	}{
		{
			name: "GivenEmptyDir_WhenScanned_ThenRootHasZeroSize",
			layout: map[string][]byte{
				"empty/": nil,
			},
			wantMinSize: 0,
			wantDirs:    1,
			wantFiles:   0,
		},
		{
			name: "GivenFlatDir_WhenScanned_ThenSizeEqualsSumOfFiles",
			layout: map[string][]byte{
				"a.txt": bytes(fileSizeSmall),
				"b.txt": bytes(fileSizeMedium),
				"c.txt": bytes(fileSizeLarge),
			},
			wantMinSize: fileSizeSmall + fileSizeMedium + fileSizeLarge,
			wantFiles:   3,
		},
		{
			name: "GivenNestedDirs_WhenScanned_ThenRootSizeIsRecursiveTotal",
			layout: map[string][]byte{
				"sub/file1.bin": bytes(fileSizeLarge),
				"sub/file2.bin": bytes(fileSizeLarge),
				"root.txt":      bytes(fileSizeSmall),
			},
			wantMinSize: fileSizeLarge*2 + fileSizeSmall,
			wantDirs:    1, // sub
			wantFiles:   1, // root.txt (at least)
		},
		{
			name:    "GivenNonExistentPath_WhenScanned_ThenReturnsError",
			layout:  nil, // don't create any dir
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var root string
			if tc.layout == nil {
				root = "/tmp/aster_does_not_exist_xyz"
			} else {
				root = makeTestDir(t, tc.layout)
			}

			node, err := scanner.Scan(context.Background(), root, nil)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Scan() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Scan() unexpected error: %v", err)
			}
			if node == nil {
				t.Fatal("Scan() returned nil node")
			}
			if node.Size() < tc.wantMinSize {
				t.Errorf("Size() = %d, want >= %d", node.Size(), tc.wantMinSize)
			}

			// Count dirs and files at the top level for basic shape validation.
			dirs, files := 0, 0
			for _, c := range node.Children {
				if c.IsDir {
					dirs++
				} else {
					files++
				}
			}
			if dirs < tc.wantDirs {
				t.Errorf("top-level dirs = %d, want >= %d", dirs, tc.wantDirs)
			}
			if files < tc.wantFiles {
				t.Errorf("top-level files = %d, want >= %d", files, tc.wantFiles)
			}
		})
	}
}

func TestScanCancellation(t *testing.T) {
	t.Run("GivenCancelledContext_WhenScanned_ThenReturnsEarly", func(t *testing.T) {
		root := makeTestDir(t, map[string][]byte{
			"a/b/c/file.bin": bytes(fileSizeLarge),
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Scan should not panic and should return some result (even possibly partial).
		// The key invariant is: no goroutine leak and no panic.
		node, _ := scanner.Scan(ctx, root, nil)
		_ = node // result may be partial; just checking it doesn't panic
	})
}

func TestScanWithProgressChannel(t *testing.T) {
	t.Run("GivenFlatDir_WhenScannedWithProgressCh_ThenProgressReceived", func(t *testing.T) {
		root := makeTestDir(t, map[string][]byte{
			"large.bin": bytes(fileSizeLarge),
		})

		progressCh := make(chan int64, 128)
		node, err := scanner.Scan(context.Background(), root, progressCh)
		close(progressCh)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var total int64
		for b := range progressCh {
			total += b
		}

		// The progress bytes emitted should equal the file size.
		if total != node.Size() {
			t.Errorf("progress total = %d, want %d (node.Size)", total, node.Size())
		}
	})
}

func TestScanSingleFile(t *testing.T) {
	root := makeTestDir(t, map[string][]byte{
		"file.bin": bytes(fileSizeMedium),
	})

	filePath := filepath.Join(root, "file.bin")
	progressCh := make(chan int64, 10)
	node, err := scanner.Scan(context.Background(), filePath, progressCh)
	close(progressCh)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node == nil || node.IsDir {
		t.Fatal("expected file node, not dir")
	}

	if node.Size() != fileSizeMedium {
		t.Errorf("Size() = %d, want %d", node.Size(), fileSizeMedium)
	}

	val := <-progressCh
	if val != fileSizeMedium {
		t.Errorf("progress = %d, want %d", val, fileSizeMedium)
	}
}

func TestScanSymlink(t *testing.T) {
	root := makeTestDir(t, map[string][]byte{
		"real_dir/file.bin": bytes(fileSizeMedium),
	})

	targetPath := filepath.Join(root, "real_dir")
	linkPath := filepath.Join(root, "symlink_dir")

	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	node, err := scanner.Scan(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node == nil {
		t.Fatal("node is nil")
	}

	foundSymlink := false
	for _, child := range node.Children {
		if child.Name == "symlink_dir" {
			foundSymlink = true
			if child.IsDir {
				t.Error("symlink should have IsDir=false to avoid deep copies")
			}
		}
	}
	if !foundSymlink {
		t.Error("did not find symlink child")
	}
}

func TestScanContextCancellationWithProgress(t *testing.T) {
	root := makeTestDir(t, map[string][]byte{
		"file1.bin": bytes(fileSizeMedium),
		"file2.bin": bytes(fileSizeMedium),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel right away

	progressCh := make(chan int64, 10)
	node, err := scanner.Scan(ctx, filepath.Join(root, "file1.bin"), progressCh)
	close(progressCh)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if node == nil {
		t.Fatal("node is nil")
	}
}

func TestGetPurgeableSpace(t *testing.T) {
	space := scanner.GetPurgeableSpace("/tmp")
	if space < 0 {
		t.Errorf("GetPurgeableSpace() returned negative value: %d", space)
	}
}
