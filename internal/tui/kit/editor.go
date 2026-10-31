package kit

import (
	"github.com/amavis442/noot/internal/editor"
	tea "github.com/charmbracelet/bubbletea"
)

// Editor opens a vault-relative file for editing and reports back with an
// EditedMsg once the user is done.
type Editor interface {
	Edit(rel string) tea.Cmd
}

// EditedMsg is sent when the editor has closed. Views reload on it, as the
// file and the index may have changed.
type EditedMsg struct {
	Rel string
	Err error
}

// External runs $VISUAL / $EDITOR in the terminal, suspending the UI meanwhile.
type External struct {
	Abs func(rel string) string // vault-relative path to absolute path
}

// Edit opens rel in the external editor.
func (x External) Edit(rel string) tea.Cmd {
	return tea.ExecProcess(editor.Command(x.Abs(rel)), func(err error) tea.Msg {
		return EditedMsg{Rel: rel, Err: err}
	})
}
