package ui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModelNewInit(t *testing.T) {
	m := New("/tmp")
	if m.state != StateScanning {
		t.Errorf("expected state StateScanning, got %v", m.state)
	}

	cmd := m.Init()
	if cmd == nil {
		t.Errorf("expected Init() to return a cmd")
	}

	// Test startScan cmd manually
	var counter atomic.Int64
	pCh := make(chan int64, 16)
	sCmd := startScan("/invalid/path/that/does/not/exist/1234", pCh, &counter)
	msg := sCmd()
	if _, ok := msg.(scanDoneMsg); !ok {
		t.Errorf("expected scanDoneMsg, got %T", msg)
	}
}

func TestSortNode(t *testing.T) {
	root := &Node{
		Name:  "root",
		IsDir: true,
	}
	root.Children = []*Node{
		{Name: "b", IsDir: false},
		{Name: "a", IsDir: true, Children: []*Node{{Name: "z"}}},
	}
	root.Children[0].SetSize(10)
	root.Children[1].SetSize(20)

	// Lazy sort: mark as unsorted then call sortNode (single level only)
	// Generation 0, mode -1 (never sorted) → IsSorted returns false.
	const gen0 = uint64(0)
	const modeSize = int8(0) // SortBySize = 0
	const modeName = int8(1) // SortByName = 1

	sortNode(root, SortBySize)
	root.MarkSorted(gen0, modeSize)
	if root.Children[0].Name != "a" {
		t.Errorf("expected 'a' to be sorted first by size")
	}

	// Simulate a generation advance (as handleSortToggle does via sortGen++).
	const gen1 = uint64(1)
	if root.IsSorted(gen1, modeName) {
		t.Errorf("expected node to be stale after generation advance")
	}
	sortNode(root, SortByName)
	root.MarkSorted(gen1, modeName)
	if root.Children[0].Name != "a" {
		t.Errorf("expected 'a' to be sorted first by name")
	}

	// ResetSorted should reset the cached generation to 0 (compat shim).
	root.ResetSorted()
	if root.IsSorted(gen1, modeName) {
		t.Errorf("expected root to be stale after ResetSorted")
	}
}

func TestModelNavAndSelection(t *testing.T) {
	m := Model{
		state: StateBrowsing,
		root: &Node{
			Name:  "root",
			IsDir: true,
			Children: []*Node{
				{Name: "foo", IsDir: true},
				{Name: "bar", IsDir: false},
			},
		},
	}
	m.clampCursor() // no crash when stack is empty

	if m.currentDir() != m.root {
		t.Errorf("expected currentDir to be root")
	}

	if len(m.visibleChildren()) != 2 {
		t.Errorf("expected 2 visible children")
	}

	if m.selected() == nil || m.selected().Name != "foo" {
		t.Errorf("expected 'foo' to be selected initially")
	}

	m.cursor = 10
	m.clampCursor()
	if m.cursor != 1 {
		t.Errorf("expected clampCursor to set cursor to 1")
	}

	m.cursor = -5
	m.clampCursor()
	if m.cursor != 0 {
		t.Errorf("expected clampCursor to set cursor to 0")
	}
}

func TestModelUpdateKeys(t *testing.T) {
	m := Model{state: StateScanning}

	// Test quit during scanning
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Errorf("expected quit cmd")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected quit cmd msg")
	}

	// Test scanDoneMsg success
	root := &Node{Name: "root", IsDir: true}
	m3, _ := m2.(Model).Update(scanDoneMsg{root: root})
	newM := m3.(Model)
	if newM.state != StateBrowsing {
		t.Errorf("expected state StateBrowsing")
	}

	// Test scanDoneMsg error
	m4, _ := m.Update(scanDoneMsg{err: errors.New("some error")})
	if m4.(Model).state != StateError {
		t.Errorf("expected state StateError")
	}

	// Test update keys while browsing
	m = Model{
		state: StateBrowsing,
		root: &Node{
			Name:  "root",
			IsDir: true,
			Children: []*Node{
				{Name: "foo", IsDir: true, Children: []*Node{{Name: "sub"}}},
				{Name: "bar", IsDir: false},
			},
		},
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// down
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(Model).cursor != 1 {
		t.Errorf("expected cursor down to move to 1")
	}

	// up
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(Model).cursor != 0 {
		t.Errorf("expected cursor up to move to 0")
	}

	// enter (nav right)
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m2.(Model).stack) != 1 {
		t.Errorf("expected to push to stack")
	}

	// backspace (nav left)
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(m2.(Model).stack) != 0 {
		t.Errorf("expected to pop stack")
	}

	// Actions
	// sort toggle
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m2.(Model).sort != SortByName {
		t.Errorf("expected sort toggle to SortByName")
	}
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m2.(Model).sort != SortBySize {
		t.Errorf("expected sort toggle to SortBySize")
	}

	// home / end
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m2.(Model).cursor != 1 {
		t.Errorf("expected G to move cursor to end")
	}
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m2.(Model).cursor != 0 {
		t.Errorf("expected g to move cursor to start")
	}
}

func TestModelUpdateConfirmDelete(t *testing.T) {
	root := &Node{
		Name:  "root",
		IsDir: true,
		Children: []*Node{
			{Name: "foo", IsDir: false},
			{Name: "bar", IsDir: false},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}
	root.Children[0].SetSize(100)
	root.AddSize(100)
	root.Children[1].SetSize(50)
	root.AddSize(50)

	m := Model{
		state: StateBrowsing,
		root:  root,
	}

	// trigger delete confirmation
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	newM := m2.(Model)
	if newM.state != StateConfirmDelete || newM.confirmPath != "root/foo" {
		t.Errorf("expected state StateConfirmDelete on root/foo, got %s", newM.confirmPath)
	}

	// abort delete
	m3, _ := newM.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newM3 := m3.(Model)
	if newM3.state != StateBrowsing {
		t.Errorf("expected state to return to StateBrowsing after esc")
	}
}

func TestModelMisc(t *testing.T) {
	m := Model{state: StateScanning, sp: spinner.New()}
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m2.(Model).width != 100 {
		t.Errorf("expected width=100")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd from WindowSizeMsg")
	}

	// spinner tick
	_, cmd = m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Errorf("expected spinner cmd")
	}
}

func TestModelView(t *testing.T) {
	// StateScanning
	m := New("/tmp")
	m.width = 80
	m.height = 24
	out := m.View()
	if len(out) == 0 {
		t.Errorf("expected View() output for scanning")
	}

	// StateError
	m.state = StateError
	m.scanErr = errors.New("my error")
	out = m.View()
	if len(out) == 0 {
		t.Errorf("expected View() output for error")
	}

	// StateBrowsing
	root := &Node{
		Name:  "sys",
		IsDir: true,
		Children: []*Node{
			{Name: "sub1", IsDir: true},
			{Name: "file1", IsDir: false},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}
	root.Children[0].SetSize(100)
	root.Children[1].SetSize(50)
	root.AddSize(150)

	m.root = root
	m.stack = append(m.stack, root)
	m.state = StateBrowsing
	m.cursor = 1
	out = m.View()
	if len(out) == 0 {
		t.Errorf("expected View() output for browsing")
	}

	// Test bounds truncation
	m.height = 2
	out = m.View()
	if len(out) == 0 {
		t.Errorf("expected View() output for small window")
	}

	// View with confirm delete
	m.state = StateConfirmDelete
	m.confirmPath = "sys/file1"
	out = m.View()
	if len(out) == 0 {
		t.Errorf("expected View() output for confirm delete")
	}

	// View with purgeable space (stack empty)
	m.state = StateBrowsing
	m.stack = nil // empty stack
	m.purgeableReady = true
	m.purgeableSpace = 1000
	m.purgeableString = "1 kB"
	out = m.View()
	if !strings.Contains(out, "purgeable:") {
		t.Errorf("expected View() output to contain purgeable info")
	}
}

func TestModelActions(t *testing.T) {
	oldTrash := trashItem
	oldOpen := openPath
	oldReveal := revealPath

	defer func() {
		trashItem = oldTrash
		openPath = oldOpen
		revealPath = oldReveal
	}()

	trashed := ""
	trashItem = func(path string) error {
		trashed = path
		return nil
	}

	opened := ""
	openPath = func(_ context.Context, path string) error {
		opened = path
		return nil
	}

	revealed := ""
	revealPath = func(_ context.Context, path string) error {
		revealed = path
		return nil
	}

	root := &Node{
		Name:  "root",
		IsDir: true,
		Children: []*Node{
			{Name: "foo", IsDir: false},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}
	root.Children[0].SetSize(100)
	root.AddSize(100)

	m := Model{
		state: StateBrowsing,
		root:  root,
	}

	// Test 'o' (open)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if opened != "root/foo" {
		t.Errorf("expected openPath to be called with root/foo, got %s", opened)
	}

	// Test 'r' (reveal)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if revealed != "root/foo" {
		t.Errorf("expected revealPath to be called with root/foo, got %s", revealed)
	}

	// Test 'd', then 'y' (confirm delete)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if trashed != "root/foo" {
		t.Errorf("expected trashItem to be called with root/foo, got %s", trashed)
	}

	if len(m3.(Model).root.Children) != 0 {
		t.Errorf("expected foo to be removed from root children")
	}
	if m3.(Model).root.Size() != 0 {
		t.Errorf("expected root size to be updated cleanly")
	}
}

func TestFetchPurgeable(t *testing.T) {
	cmd := fetchPurgeable("/tmp")
	msg := cmd()
	pMsg, ok := msg.(purgeableSpaceMsg)
	if !ok {
		t.Fatalf("expected purgeableSpaceMsg, got %T", msg)
	}
	if pMsg.space < 0 {
		t.Errorf("expected non-negative space, got %d", pMsg.space)
	}
	if pMsg.str == "" {
		t.Errorf("expected non-empty string for purgeable space")
	}
}

func TestModelUpdatePurgeable(t *testing.T) {
	m := Model{}
	m2, _ := m.Update(purgeableSpaceMsg{space: 1024, str: "1 kB"})
	newM := m2.(Model)
	if !newM.purgeableReady {
		t.Errorf("expected purgeableReady to be true")
	}
	if newM.purgeableSpace != 1024 {
		t.Errorf("expected purgeableSpace to be 1024")
	}
	if newM.purgeableString != "1 kB" {
		t.Errorf("expected purgeableString to be '1 kB'")
	}
}
