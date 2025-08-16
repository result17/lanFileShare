package multiFilePicker

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDir creates a temporary directory structure for testing.
// It returns the path to the temp directory and a cleanup function.
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp(".", "test-picker-*")
	require.NoError(t, err, "Failed to create temp dir")

	// Create some files and directories with predictable names for sorting
	err = os.WriteFile(filepath.Join(tempDir, "file_a.txt"), []byte("a"), 0666)
	require.NoError(t, err, "Failed to create file_a.txt")
	
	err = os.Mkdir(filepath.Join(tempDir, "subdir_b"), 0777)
	require.NoError(t, err, "Failed to create subdir_b")
	
	err = os.WriteFile(filepath.Join(tempDir, "subdir_b", "file_c.txt"), []byte("c"), 0666)
	require.NoError(t, err, "Failed to create subdir_b/file_c.txt")
	
	err = os.WriteFile(filepath.Join(tempDir, "file_d.txt"), []byte("d"), 0666)
	require.NoError(t, err, "Failed to create file_d.txt")

	cleanup := func() {
		err := os.RemoveAll(tempDir)
		assert.NoError(t, err, "Failed to clean up temp dir")
	}

	return tempDir, cleanup
}

func TestInitialModel(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	err := m.SetPath(tempDir)
	require.NoError(t, err, "failed to set path")

	absPath, err := filepath.Abs(tempDir)
	require.NoError(t, err, "invalid temp dir %s", tempDir)

	assert.Equal(t, absPath, m.path, "path should match absolute path")

	// Expecting 3 items: file_a.txt, file_d.txt, subdir_b
	assert.Len(t, m.items, 3, "should have 3 items")
	assert.Empty(t, m.selected, "selected map should be empty initially")
}

func TestUpdateMovement(t *testing.T) {
	m := Model{
		items: make([]displayItem, 3), // 3 dummy items
		keys:  DefaultKeyMap,
	}

	// Use a helper to create key messages
	keyDown := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	keyUp := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Test moving down
	newModel, _ := m.Update(keyDown)
	m = newModel.(Model)
	assert.Equal(t, 1, m.cursor, "cursor should be at 1 after moving down")

	// Test moving down again
	newModel, _ = m.Update(keyDown)
	m = newModel.(Model)
	assert.Equal(t, 2, m.cursor, "cursor should be at 2 after moving down")

	// Test moving down at the bottom
	newModel, _ = m.Update(keyDown)
	m = newModel.(Model)
	assert.Equal(t, 2, m.cursor, "cursor should stay at 2 at the bottom")

	// Test moving up
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	assert.Equal(t, 1, m.cursor, "cursor should be at 1 after moving up")

	// Test moving up again
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	assert.Equal(t, 0, m.cursor, "cursor should be at 0 after moving up")

	// Test moving up at the top
	newModel, _ = m.Update(keyUp)
	m = newModel.(Model)
	assert.Equal(t, 0, m.cursor, "cursor should stay at 0 at the top")
}

func TestUpdateSelection(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	err := m.SetPath(tempDir)
	require.NoError(t, err, "Failed to set path")

	spaceKey := tea.KeyMsg{Type: tea.KeySpace}

	// Initially no items should be selected
	assert.Empty(t, m.selected, "initially no items should be selected")

	// Select item at cursor 0 (should be 'subdir_b' since directories come first)
	newModel, _ := m.Update(spaceKey)
	m = newModel.(Model)

	// Verify the item is selected
	item0Path := m.items[0].Path
	assert.Contains(t, m.selected, item0Path, "item 0 should be selected")
	assert.Len(t, m.selected, 1, "should have exactly 1 item selected")

	// Select the same item again (should deselect it)
	newModel, _ = m.Update(spaceKey)
	m = newModel.(Model)

	assert.NotContains(t, m.selected, item0Path, "item 0 should be deselected")
	assert.Empty(t, m.selected, "should have no items selected after deselecting")
}

func TestConfirmSelection(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	m := InitialModel()

	err := m.SetPath(tempDir)
	require.NoError(t, err, "Failed to set path")

	absPath, err := filepath.Abs(tempDir)
	require.NoError(t, err, "invalid temp dir %s", tempDir)

	// Sorted items are: subdir_b (directory first), file_a.txt, file_d.txt

	// 1. Select 'subdir_b' (cursor is at 0 since directories come first)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newModel.(Model)

	// 2. Move cursor to 'file_a.txt' (index 1)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newModel.(Model)
	assert.Equal(t, 1, m.cursor, "cursor should be at index 1")

	// 3. Select 'file_a.txt'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newModel.(Model)

	// Verify we have 2 items selected
	assert.Len(t, m.selected, 2, "should have 2 items selected")

	// 4. Confirm selection - this should return a command that sends SelectedFileNodeMsg
	finalModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = finalModel.(Model)

	// Execute the command to get the message
	if cmd != nil {
		msg := cmd()
		if selectedMsg, ok := msg.(SelectedFileNodeMsg); ok {
			assert.Len(t, selectedMsg.Files, 2, "should have 2 files in the message")
			
			// Check that the selected files are correct
			selectedPaths := make([]string, len(selectedMsg.Files))
			for i, file := range selectedMsg.Files {
				selectedPaths[i] = file.Path
			}
			sort.Strings(selectedPaths)

			expectedPaths := []string{
				filepath.Join(absPath, "subdir_b"),
				filepath.Join(absPath, "file_a.txt"),
			}
			sort.Strings(expectedPaths)

			assert.Equal(t, expectedPaths, selectedPaths, "selected file paths should match expected")
		} else {
			t.Errorf("expected SelectedFileNodeMsg, got %T", msg)
		}
	} else {
		t.Error("expected command to be returned, got nil")
	}
}

func TestQuit(t *testing.T) {
	m := Model{keys: DefaultKeyMap}
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
