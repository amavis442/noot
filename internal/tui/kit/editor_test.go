package kit_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/amavis442/noot/internal/tui/kit"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// fakeEditorEnv makes this test binary act as $VISUAL: it appends a line to
// the file it is given, as a user editing it would.
const fakeEditorEnv = "NOOT_FAKE_EDITOR"

func TestMain(m *testing.M) {
	if os.Getenv(fakeEditorEnv) != "" {
		f := os.Args[len(os.Args)-1]
		b, err := os.ReadFile(f)
		if err == nil {
			err = os.WriteFile(f, append(b, []byte("- [ ] erbij\r\n")...), 0o644)
		}
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// editOnce asks External to edit a text and quits with what came back.
type editOnce struct {
	got kit.TextEditedMsg
}

func (e *editOnce) Init() tea.Cmd {
	return kit.External{}.EditText("2026-11-02", "- [ ] Taak")
}

func (e *editOnce) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if got, ok := msg.(kit.TextEditedMsg); ok {
		e.got = got
		return e, tea.Quit
	}
	return e, nil
}

func (e *editOnce) View() string { return "" }

func TestExternalEditsATextThroughATemporaryFile(t *testing.T) {
	t.Setenv("VISUAL", os.Args[0])
	t.Setenv(fakeEditorEnv, "1")
	m := &editOnce{}
	tm := teatest.NewTestModel(t, m)
	tm.WaitFinished(t, teatest.WithFinalTimeout(20*time.Second))
	got := m.got
	if got.Err != nil || got.Name != "2026-11-02" || got.Content != "- [ ] Taak\n- [ ] erbij" {
		t.Fatalf("edited = %+v", got)
	}
	left, _ := os.ReadDir(os.TempDir())
	for _, f := range left {
		if strings.HasPrefix(f.Name(), "noot-2026-11-02-") {
			t.Fatalf("temporary file %s left behind", f.Name())
		}
	}
}
