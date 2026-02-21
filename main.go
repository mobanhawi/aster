package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mobanhawi/aster/internal/ui"
)

var version = "dev"

var runProgram = func(p *tea.Program) (tea.Model, error) { // injected for testing
	return p.Run()
}

var osExit = os.Exit

func main() {
	osExit(run(os.Args))
}

func run(args []string) int {
	if len(args) >= 2 && (args[1] == "-v" || args[1] == "--version") {
		fmt.Printf("aster version %s\n", version)
		return 0
	}

	if len(args) >= 2 && (args[1] == "-h" || args[1] == "--help") {
		fmt.Println("usage: aster <path>")
		fmt.Println("       aster ~/Downloads")
		return 0
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: aster <path>")
		fmt.Fprintln(os.Stderr, "       aster ~/Downloads")
		return 1
	}

	root := args[1]

	// Resolve to absolute path
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving path: %v\n", err)
		return 1
	}

	// Verify the path exists and is bounded securely
	cleanRoot := filepath.Clean(absRoot)
	if filepath.VolumeName(cleanRoot) != "" {
		cleanRoot = filepath.VolumeName(cleanRoot) + filepath.FromSlash(cleanRoot)
	}

	cleanRootAbs, err := filepath.Abs(cleanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error computing valid path: %v\n", err)
		return 1
	}

	// #nosec G703 -- This is a CLI. Exploring untrusted paths directly from input is intended.
	if _, err := os.Stat(cleanRootAbs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	model := ui.New(absRoot)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := runProgram(p); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		return 1
	}
	return 0
}
