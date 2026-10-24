package vault

import "testing"

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Clean architectuur in Go!": "clean-architectuur-in-go",
		"  Café über naïef  ":       "cafe-uber-naief",
		"Wat is een Zettel?":        "wat-is-een-zettel",
		"!!!":                       "",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
