package multiFilePicker

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// setupTestDir creates a temporary directory structure for testing.
// It returns the path to the temp directory and a cleanup function.
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp(".", "test-picker-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create some files and directories with predictable names for sorting
	if err := os.WriteFile(filepath.Join(tempDir, "file_a.txt"), []byte("a"), 0666); err != nil {
		t.Fatalf("Failed to create file_a.txt: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "subdir_b"), 0777); err != nil {
		t.Fatalf("Failed to create subdir_b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "subdir_b", "file_c.txt"), []byte("c"), 0666); err != nil {
		t.Fatalf("Failed to create subdir_b/file_c.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "file_d.txt"), []byte("d"), 0666); err != nil {
		t.Fatalf("Failed to create file_d.txt: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestInitialModel(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	m.SetPath(tempDir)

	absPath, err := filepath.Abs(tempDir)

	if err != nil {
		t.Errorf("invalid temp dir %s", tempDir)
	}

	if m.path != absPath {
		t.Errorf("expected path %q, got %q", absPath, m.path)
	}

	// Expecting 3 items: file_a.txt, file_d.txt, subdir_b
	if len(m.items) != 3 {
		t.Errorf("expected 3 items, got %d", len(m.items))
	}

	if len(m.selected) != 0 {
		t.Errorf("expected selected map to be empty, but it has %d items", len(m.selected))
	}
}

func TestUpdateMovement(t *testing.T) {
	m := Model{
		items: make([]fs.DirEntry, 3), // 3 dummy items
		keys:  DefaultKeyMap,
	}

	// Use a helper to create key messages
	keyDown := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	keyUp := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Test moving down
	newModel, _ := m.Update(keyDown)
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor to be at 1 after moving down, got %d", m.cursor)
	}

	// Test moving down again
	newModel, _ = m.Update(keyDown)
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor to be at 2 after moving down, got %d", m.cursor)
	}

	// Test moving down at the bottom
	newModel, _ = m.Update(keyDown)
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor to stay at 2 at the bottom, got %d", m.cursor)
	}

	// Test moving up
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor to be at 1 after moving up, got %d", m.cursor)
	}

	// Test moving up again
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor to be at 0 after moving up, got %d", m.cursor)
	}

	// Test moving up at the top
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 at the top, got %d", m.cursor)
	}
}

func TestUpdateSelection(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	m.SetPath(tempDir)

	spaceKey := tea.KeyMsg{Type: tea.KeySpace}
	enterKey := tea.KeyMsg{Type: tea.KeyEnter}

	// Select item at cursor 0 ('file_a.txt')
	newModel, _ := m.Update(spaceKey)
	newModel, _ = m.Update(enterKey)
	m = newModel.(Model)
	absPath, err := filepath.Abs(tempDir)

	if err != nil {
		t.Errorf("invalid temp dir %s", tempDir)
	}
	item0Path := filepath.Join(absPath, m.items[0].Name())
	if _, ok := m.selected[item0Path]; !ok {
		t.Errorf("expected item 0 (%s) to be selected, but it's not. selected is %v, selected size is %d", item0Path, m.selected, len(m.selected))
	}
}

func TestConfirmSelection(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	m.SetPath(tempDir)

	absPath, err := filepath.Abs(tempDir)

	if err != nil {
		t.Errorf("invalid temp dir %s", tempDir)
	}

	// Sorted items are: file_a.txt, file_d.txt, subdir_b

	// 1. Select 'file_a.txt' (cursor is at 0)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newModel.(Model)

	// 2. Move cursor to 'subdir_b' (index 2)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Fatalf("cursor should be at index 2, but is at %d", m.cursor)
	}

	// 3. Select 'subdir_b'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newModel.(Model)

	// 4. Confirm selection
	finalModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	m = finalModel.(Model)

	expected := []string{
		filepath.Join(absPath, "file_a.txt"),
		filepath.Join(absPath, "subdir_b", "file_c.txt"),
	}

	// Sort both slices for consistent comparison
	sort.Strings(expected)

	for i, info := range m.files {
		if info.Path != expected[i] {
			t.Errorf("expected path to be %v, got %v", expected[i], info.Path)
		}

	}
}

func TestQuit(t *testing.T) {
	m := Model{keys: DefaultKeyMap}

	// Test with 'esc'
	m = Model{keys: DefaultKeyMap} // reset model
	finalModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !finalModel.(Model).quitting {
		t.Errorf("expected model to be quitting after 'esc' key, but it's not")
	}

	// Test with 'ctrl+c'
	m = Model{keys: DefaultKeyMap} // reset model
	finalModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !finalModel.(Model).quitting {
		t.Errorf("expected model to be quitting after 'ctrl+c' key, but it's not")
	}
}
