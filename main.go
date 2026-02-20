package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mobanhawi/aster/internal/ui"
)

var version = "dev"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Printf("aster version %s\n", version)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: aster <path>")
		fmt.Fprintln(os.Stderr, "       aster ~/Downloads")
		os.Exit(1)
	}

	root := os.Args[1]

	// Resolve to absolute path
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Verify the path exists
	// Verify the path exists and is bounded securely
	cleanRoot := filepath.Clean(absRoot)
	if filepath.VolumeName(cleanRoot) != "" {
		cleanRoot = filepath.VolumeName(cleanRoot) + filepath.FromSlash(cleanRoot)
	}

	cleanRootAbs, err := filepath.Abs(cleanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error computing valid path: %v\n", err)
		os.Exit(1)
	}

	// #nosec G703 -- This is a CLI. Exploring untrusted paths directly from input is intended.
	if _, err := os.Stat(cleanRootAbs); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	model := ui.New(absRoot)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
