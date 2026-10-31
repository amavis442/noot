package cli

import (
	"io"
	"os"
	"strings"

	"github.com/amavis442/noot/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// painter colours command output in the theme of the terminal UI. When it is
// off it returns text unchanged, so the output stays what scripts expect.
type painter struct {
	on                                              bool
	accent, bold, faint, red, green, yellow, strong lipgloss.Style
}

// newPainter returns a painter rendering with r; on is false for plain text.
func newPainter(r *lipgloss.Renderer, on bool) painter {
	s := r.NewStyle
	return painter{
		on:     on,
		accent: s().Foreground(theme.Accent),
		bold:   s().Bold(true),
		faint:  s().Foreground(theme.Muted),
		red:    s().Foreground(theme.Danger),
		green:  s().Foreground(theme.Success),
		yellow: s().Foreground(theme.Warning),
		strong: s().Bold(true).Foreground(theme.Accent),
	}
}

// styledOutput reports whether output to w gets colours: only on a terminal,
// without --json and without NO_COLOR (https://no-color.org).
func styledOutput(opts *options, w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && !opts.asJSON && os.Getenv("NO_COLOR") == "" && isTerminal(f)
}

// painterFor returns the painter for output to w, and a function that undoes
// what turning colours on changed in the console.
func painterFor(opts *options, w io.Writer) (painter, func()) {
	if !styledOutput(opts, w) {
		return newPainter(lipgloss.NewRenderer(w), false), func() {}
	}
	out := termenv.NewOutput(w)
	restore, err := termenv.EnableVirtualTerminalProcessing(out) // Windows consoles
	if err != nil {
		return newPainter(lipgloss.NewRenderer(w), false), func() {}
	}
	return newPainter(lipgloss.NewRenderer(w), true), func() { _ = restore() }
}

func (p painter) paint(st lipgloss.Style, text string) string {
	if !p.on || text == "" {
		return text
	}
	return st.Render(text)
}

func (p painter) path(s string) string    { return p.paint(p.accent, s) }
func (p painter) title(s string) string   { return p.paint(p.bold, s) }
func (p painter) muted(s string) string   { return p.paint(p.faint, s) }
func (p painter) danger(s string) string  { return p.paint(p.red, s) }
func (p painter) success(s string) string { return p.paint(p.green, s) }
func (p painter) warning(s string) string { return p.paint(p.yellow, s) }

// snippet dims a search snippet and makes the «matches» stand out.
func (p painter) snippet(s string) string {
	if !p.on {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "«")
		end := strings.Index(s, "»")
		if start < 0 || end < start {
			b.WriteString(p.muted(s))
			return b.String()
		}
		end += len("»")
		b.WriteString(p.muted(s[:start]))
		b.WriteString(p.paint(p.strong, s[start:end]))
		s = s[end:]
	}
}
