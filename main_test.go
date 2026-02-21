package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRun(t *testing.T) {
	// Backup original runProgram and restore after test
	originalRunProgram := runProgram
	defer func() { runProgram = originalRunProgram }()

	tempDir := t.TempDir()

	tests := []struct {
		name         string
		args         []string
		mockRunError error
		expectedCode int
	}{
		{
			name:         "version flag -v",
			args:         []string{"aster", "-v"},
			expectedCode: 0,
		},
		{
			name:         "version flag --version",
			args:         []string{"aster", "--version"},
			expectedCode: 0,
		},
		{
			name:         "help flag -h",
			args:         []string{"aster", "-h"},
			expectedCode: 0,
		},
		{
			name:         "help flag --help",
			args:         []string{"aster", "--help"},
			expectedCode: 0,
		},
		{
			name:         "no args",
			args:         []string{"aster"},
			expectedCode: 1,
		},
		{
			name:         "invalid path",
			args:         []string{"aster", filepath.Join(tempDir, "does-not-exist")},
			expectedCode: 1,
		},
		{
			name:         "valid path success",
			args:         []string{"aster", tempDir},
			expectedCode: 0,
		},
		{
			name:         "valid path tea program error",
			args:         []string{"aster", tempDir},
			mockRunError: errors.New("tea program failed"),
			expectedCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runProgram = func(_ *tea.Program) (tea.Model, error) {
				return nil, tt.mockRunError
			}

			// redirect stderr/stdout if needed, but for coverage it's ok as is
			// to avoid noisy test output we could capture it, but simple tests are fine

			// backup stderr & stdout
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			defer func() {
				os.Stdout = oldStdout
				os.Stderr = oldStderr
			}()

			// pipe output to dev null
			nullOut, err := os.Open(os.DevNull)
			if err == nil {
				os.Stdout = nullOut
				os.Stderr = nullOut
				defer nullOut.Close()
			}

			code := run(tt.args)
			if code != tt.expectedCode {
				t.Errorf("expected exit code %d, got %d", tt.expectedCode, code)
			}
		})
	}
}

func TestMainFunc(t *testing.T) {
	// mock os.Args, osExit
	originalArgs := os.Args
	originalRunProgram := runProgram
	originalOsExit := osExit

	defer func() {
		os.Args = originalArgs
		runProgram = originalRunProgram
		osExit = originalOsExit
	}()

	os.Args = []string{"aster", "-v"}
	runProgram = func(_ *tea.Program) (tea.Model, error) {
		return nil, nil
	}

	exitedWith := -1
	osExit = func(code int) {
		exitedWith = code
	}

	main()

	if exitedWith != 0 {
		t.Errorf("expected main to exit with 0, got %d", exitedWith)
	}
}
