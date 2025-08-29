
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestKeyboardManager_ContextPriority(t *testing.T) {
	km := NewKeyboardManager()

	// Given: A global binding and a context-specific binding for the same key
	km.AddGlobalBinding(KeyBinding{
		Keys:   []string{"r"},
		Action: KeyActionRefresh,
	})
	km.AddContextBinding("test_context", KeyBinding{
		Keys:   []string{"r"},
		Action: KeyActionResume,
		Enabled: true,
	})

	// When: The context is set and the key is processed
	km.SetContext("test_context")
	action := km.ProcessKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	// Then: The context-specific action should be returned
	assert.Equal(t, KeyActionResume, action, "Context-specific binding should have priority")
}

func TestKeyboardManager_GlobalBindings(t *testing.T) {
	km := NewKeyboardManager()

	// Given: A global binding for 'q'
	// This is added by default, but we can ensure it's there
	km.AddGlobalBinding(KeyBinding{
		Keys:   []string{"q"},
		Action: KeyActionQuit,
		Enabled: true,
	})

	// When: The context is arbitrary and the key is processed
	km.SetContext("any_context")
	action := km.ProcessKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Then: The global action should be returned
	assert.Equal(t, KeyActionQuit, action, "Global binding should work in any context")
}

func TestKeyboardManager_EnableDisableBinding(t *testing.T) {
	km := NewKeyboardManager()
	
	// Given: A binding in a specific context
	km.AddContextBinding("test_context", KeyBinding{
		Keys:    []string{"p"},
		Action:  KeyActionPause,
		Enabled: true,
	})
	km.SetContext("test_context")

	// When: The binding is disabled
	km.EnableBinding([]string{"p"}, false)
	action := km.ProcessKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	// Then: No action should be triggered
	assert.Equal(t, KeyActionNone, action, "Disabled binding should not trigger an action")

	// When: The binding is re-enabled
	km.EnableBinding([]string{"p"}, true)
	action = km.ProcessKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	// Then: The action should be triggered
	assert.Equal(t, KeyActionPause, action, "Re-enabled binding should trigger an action")
}

func TestKeyboardManager_ProcessKey_NoMatch(t *testing.T) {
	km := NewKeyboardManager()

	// Given: A keyboard manager with default bindings
	km.SetContext("discovery")

	// When: An unbound key is processed
	action := km.ProcessKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Then: KeyActionNone should be returned
	assert.Equal(t, KeyActionNone, action, "Processing an unbound key should result in no action")
}

func TestKeyboardManager_GetActiveBindings(t *testing.T) {
	km := NewKeyboardManager()

	// Given: A mix of global and context-specific bindings
	km.AddGlobalBinding(KeyBinding{Keys: []string{"g"}, Action: KeyActionQuit, Enabled: true, Global: true})
	km.AddContextBinding("test_context", KeyBinding{Keys: []string{"c"}, Action: KeyActionConfirm, Enabled: true})
	km.AddContextBinding("test_context", KeyBinding{Keys: []string{"d"}, Action: KeyActionCancel, Enabled: false}) // Disabled binding

	// When: The context is set
	km.SetContext("test_context")
	activeBindings := km.GetActiveBindings()

	// Then: The correct number of active bindings should be returned
	// Note: This counts default global bindings + the ones we added.
	// Default globals: q, ?, f11, ctrl+r. We added 'g'. Total 5.
	// Context specific: 'c'. 'd' is disabled. Total 1.
	// Grand total should be 5 + 1 = 6
	
	// Let's create a fresh manager to have a predictable count
	km = &KeyboardManager{
		contextBindings: make(map[string][]KeyBinding),
		globalBindings:  make([]KeyBinding, 0),
	}
	km.AddGlobalBinding(KeyBinding{Keys: []string{"g"}, Action: KeyActionQuit, Enabled: true, Global: true})
	km.AddContextBinding("test_context", KeyBinding{Keys: []string{"c"}, Action: KeyActionConfirm, Enabled: true})
	km.AddContextBinding("test_context", KeyBinding{Keys: []string{"d"}, Action: KeyActionCancel, Enabled: false})
	km.SetContext("test_context")
	activeBindings = km.GetActiveBindings()

	assert.Len(t, activeBindings, 2, "Should return 1 global and 1 active context binding")

	actionMap := make(map[KeyAction]bool)
	for _, b := range activeBindings {
		actionMap[b.Action] = true
	}

	assert.True(t, actionMap[KeyActionQuit], "Active global binding should be present")
	assert.True(t, actionMap[KeyActionConfirm], "Active context binding should be present")
	assert.False(t, actionMap[KeyActionCancel], "Disabled context binding should not be present")
}
