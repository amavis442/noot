package theme_test

import (
	"testing"

	"github.com/amavis442/noot/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

func TestEveryColourHasALightAndADarkShade(t *testing.T) {
	colours := map[string]lipgloss.AdaptiveColor{
		"Accent": theme.Accent, "Muted": theme.Muted, "Danger": theme.Danger,
		"Success": theme.Success, "Warning": theme.Warning, "Panel": theme.Panel,
	}
	for name, c := range colours {
		if c.Light == "" || c.Dark == "" || c.Light == c.Dark {
			t.Errorf("%s = %+v, want distinct light and dark shades", name, c)
		}
	}
}
