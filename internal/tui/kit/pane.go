package kit

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// paneChrome is the heading line and the blank line under it.
const paneChrome = 2

// Pane shows a note rendered as Markdown under a heading, scrollable with the
// list and page keys.
type Pane struct {
	md       Markdown
	vp       viewport.Model
	heading  string
	sub      string
	source   string
	rendered string
	err      error
	width    int
}

// NewPane returns an empty pane rendering with md.
func NewPane(md Markdown) *Pane {
	return &Pane{md: md, vp: viewport.New(0, 0)}
}

// SetSize sets the room the pane may use, heading included.
func (p *Pane) SetSize(width, height int) {
	p.width = width
	p.vp.Width = width
	p.vp.Height = max(height-paneChrome, 1)
	p.render()
}

// SetHeading sets the bold heading and the muted text after it.
func (p *Pane) SetHeading(heading, sub string) { p.heading, p.sub = heading, sub }

// SetMarkdown replaces the content and scrolls back to the top.
func (p *Pane) SetMarkdown(src string) {
	p.source = src
	p.render()
	p.vp.GotoTop()
}

// SetError shows err instead of the content; nil shows the content again.
func (p *Pane) SetError(err error) { p.err = err }

// ScrollTo scrolls the first rendered line containing text to the top.
func (p *Pane) ScrollTo(text string) {
	for i, line := range strings.Split(ansi.Strip(p.rendered), "\n") {
		if strings.Contains(line, text) {
			p.vp.SetYOffset(i)
			return
		}
	}
}

// Scroll scrolls on the list and page keys and reports whether msg was one.
// The viewport's own keys are not used: they would take h, l and f, which
// mean something else here.
func (p *Pane) Scroll(msg tea.KeyMsg) bool {
	switch {
	case key.Matches(msg, Keys.ListUp):
		p.vp.ScrollUp(1)
	case key.Matches(msg, Keys.ListDown):
		p.vp.ScrollDown(1)
	case key.Matches(msg, Keys.PageUp):
		p.vp.PageUp()
	case key.Matches(msg, Keys.PageDown):
		p.vp.PageDown()
	default:
		return false
	}
	return true
}

func (p *Pane) render() {
	out, err := p.md.Render(p.source, p.width)
	if err != nil {
		out = p.source
	}
	p.rendered = out
	p.vp.SetContent(out)
}

// View draws the heading over the content, or over the error.
func (p *Pane) View() string {
	head := Heading.Render(p.heading)
	if p.sub != "" {
		head += Muted.Render("  " + p.sub)
	}
	body := p.vp.View()
	if p.err != nil {
		body = Error.Render(p.err.Error())
	}
	return lipgloss.JoinVertical(lipgloss.Left, Truncate(head, max(p.width, 1)), "", body)
}
