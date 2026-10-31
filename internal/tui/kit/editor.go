package kit

import (
	"os"
	"strings"

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

// TextEditor opens a piece of text, such as one day of the agenda, for
// editing and reports back with a TextEditedMsg holding the new text.
type TextEditor interface {
	EditText(name, content string) tea.Cmd
}

// TextEditedMsg is sent when the editor of a text has closed. Content is the
// text as saved, with Windows line endings and trailing newlines removed.
type TextEditedMsg struct {
	Name    string
	Content string
	Err     error
}

// EditText writes content to a temporary file named after name, opens it in
// the external editor and reads it back; the file is removed afterwards.
func (x External) EditText(name, content string) tea.Cmd {
	f, err := os.CreateTemp("", "noot-"+name+"-*.md")
	if err == nil {
		_, err = f.WriteString(content + "\n")
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		return func() tea.Msg { return TextEditedMsg{Name: name, Err: err} }
	}
	return tea.ExecProcess(editor.Command(f.Name()), func(err error) tea.Msg {
		defer os.Remove(f.Name())
		if err != nil {
			return TextEditedMsg{Name: name, Err: err}
		}
		b, err := os.ReadFile(f.Name())
		text := strings.TrimRight(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
		return TextEditedMsg{Name: name, Content: text, Err: err}
	})
}

// Editing opens both files and texts, as External does.
type Editing interface {
	Editor
	TextEditor
}
