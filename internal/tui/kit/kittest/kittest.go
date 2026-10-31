// Package kittest helps unit-test views: it turns key names into key messages,
// runs a view's commands and feeds their messages back, as Bubble Tea would,
// and offers an editor that only records what it was asked to open.
package kittest

import (
	"testing"

	"github.com/amavis442/noot/internal/tui/kit"
	tea "github.com/charmbracelet/bubbletea"
)

// maxSteps guards against commands that keep producing messages.
const maxSteps = 50

// Key returns the key message for a key name such as "j", "enter" or "ctrl+n".
// Anything that is not a named key is typed as text.
func Key(name string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"enter": tea.KeyEnter, "esc": tea.KeyEsc, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "backspace": tea.KeyBackspace,
		"ctrl+c": tea.KeyCtrlC, "ctrl+n": tea.KeyCtrlN, "ctrl+p": tea.KeyCtrlP, "space": tea.KeySpace,
	}
	if t, ok := named[name]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// Send passes msg to v and runs the resulting commands until none are left.
func Send(t *testing.T, v kit.View, msg tea.Msg) {
	t.Helper()
	Run(t, v, v.Update(msg))
}

// Type sends every key name in turn; text such as "rogge" is typed rune by rune.
func Type(t *testing.T, v kit.View, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if key := Key(k); key.Type != tea.KeyRunes {
			Send(t, v, key)
			continue
		}
		for _, r := range k {
			Send(t, v, Key(string(r)))
		}
	}
}

// Run runs cmd and feeds every message it produces back into v, until no
// command is left.
func Run(t *testing.T, v kit.View, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > maxSteps {
			t.Fatal("commands keep producing messages")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		default:
			queue = append(queue, v.Update(msg))
		}
	}
}

// Editor records the files it is asked to edit and reports them edited at once.
type Editor struct {
	Opened []string
}

// Edit records rel and returns its EditedMsg.
func (e *Editor) Edit(rel string) tea.Cmd {
	e.Opened = append(e.Opened, rel)
	return func() tea.Msg { return kit.EditedMsg{Rel: rel} }
}
