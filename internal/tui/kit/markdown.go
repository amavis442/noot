// Package kit holds what the views of the terminal UI share: the View
// contract, the theme, Markdown rendering and opening the external editor.
package kit

import (
	"strings"

	"github.com/amavis442/noot/internal/parser"
	"github.com/charmbracelet/glamour"
)

// Markdown renders a note's content for the terminal, width columns wide.
type Markdown interface {
	Render(content string, width int) (string, error)
}

// Glamour renders Markdown with a glamour style: "dark", "light" or "notty"
// (no colours). Renderers are kept per width, as building one is not cheap.
type Glamour struct {
	style     string
	renderers map[int]*glamour.TermRenderer
}

// NewGlamour returns a renderer using the named glamour style.
func NewGlamour(style string) *Glamour {
	return &Glamour{style: style, renderers: map[int]*glamour.TermRenderer{}}
}

// glamourMargin is the room glamour's styles take around the text.
const glamourMargin = 4

// Render renders content without its frontmatter, wrapped to width.
func (g *Glamour) Render(content string, width int) (string, error) {
	wrap := max(width-glamourMargin, 10)
	r, ok := g.renderers[wrap]
	if !ok {
		var err error
		r, err = glamour.NewTermRenderer(glamour.WithStandardStyle(g.style), glamour.WithWordWrap(wrap))
		if err != nil {
			return "", err
		}
		g.renderers[wrap] = r
	}
	out, err := r.Render(string(parser.Body([]byte(content))))
	if err != nil {
		return "", err
	}
	return strings.Trim(out, "\n"), nil
}

// Plain shows the Markdown source without frontmatter, for tests.
type Plain struct{}

// Render returns content without its frontmatter.
func (Plain) Render(content string, _ int) (string, error) {
	return string(parser.Body([]byte(content))), nil
}
