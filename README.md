# aster

A terminal disk usage analyzer for macOS. Navigate your filesystem, identify large directories, and clean up — all from the terminal.

[![CI](https://github.com/mobanhawi/aster/actions/workflows/go.yml/badge.svg)](https://github.com/mobanhawi/aster/actions/workflows/go.yml)
[![Release](https://github.com/mobanhawi/aster/actions/workflows/release.yml/badge.svg)](https://github.com/mobanhawi/aster/actions/workflows/release.yml)
[![codecov](https://codecov.io/gh/mobanhawi/aster/branch/main/graph/badge.svg)](https://codecov.io/gh/mobanhawi/aster)

![aster avatar](./assets/gopher-aster.png)

## TUI Screenshot
![aster TUI](./assets/demo.gif)

## Install

### Homebrew (recommended)

```bash
brew tap mobanhawi/aster
brew install aster
```

### Build from source

```bash
git clone https://github.com/mobanhawi/aster
cd aster
go build -o aster .
```

## Usage

```bash
./aster <path>
./aster ~/Downloads
./aster /
```

## Keys

| Key | Action |
|-----|--------|
| `j` / `k` or arrows | Move cursor |
| `enter` / `l` | Enter directory |
| `backspace` / `h` | Go back |
| `s` | Toggle sort (size / name) |
| `o` | Open item in default app |
| `r` | Show item's location in Finder |
| `d` | Move to Trash (with confirm) |
| `g` / `G` | Jump to top / bottom |
| `q` | Quit |

*Note: `o` (Open) launches the item itself. `r` (Reveal) opens the folder containing the item and highlights it.*

## Requirements

- macOS
- Go 1.21+
