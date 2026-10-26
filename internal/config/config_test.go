package config

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("NOOT_CONFIG", filepath.Join(t.TempDir(), "sub", "config.yaml"))
	c, err := Load()
	if err != nil || len(c.Vaults) != 0 {
		t.Fatalf("empty load: %+v %v", c, err)
	}
	dir := t.TempDir()
	if err := c.Add("patrick", dir, KindPersonal); err != nil {
		t.Fatal(err)
	}
	if err := c.Add("claude", filepath.Join(dir, "agent"), KindAgent); err != nil {
		t.Fatal(err)
	}
	if c.Default != "patrick" || c.Agent != "claude" {
		t.Errorf("default/agent = %q/%q", c.Default, c.Agent)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	c2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := c2.Get("claude"); !ok || v.Kind != KindAgent {
		t.Errorf("claude = %+v %v", v, ok)
	}
	if c2.NameFor(dir) != "patrick" {
		t.Errorf("NameFor = %q", c2.NameFor(dir))
	}
	if err := c2.Remove("patrick"); err != nil || c2.Default != "" {
		t.Errorf("remove: %v default=%q", err, c2.Default)
	}
}

func TestRegister(t *testing.T) {
	c := &Config{Vaults: map[string]Vault{}}
	root := t.TempDir()
	home, work := filepath.Join(root, "home", "Notes"), filepath.Join(root, "work", "Notes")

	// Without a name: the fallback (the folder name).
	if n, err := c.Register("", "notes", home, KindPersonal); err != nil || n != "notes" || c.Default != "notes" {
		t.Fatalf("Register = %q, %v (default %q)", n, err, c.Default)
	}
	// Again for the same folder: it keeps its name, whatever the fallback.
	if n, err := c.Register("", "other", home, KindPersonal); err != nil || n != "notes" || len(c.Vaults) != 1 {
		t.Errorf("re-register = %q, %v, %v", n, err, c.Names())
	}
	// Another folder with the same name is refused instead of overwriting.
	if _, err := c.Register("", "notes", work, KindPersonal); err == nil {
		t.Error("a taken fallback name should fail")
	}
	if _, err := c.Register("notes", "", work, KindPersonal); err == nil {
		t.Error("a taken explicit name should fail")
	}
	if v, _ := c.Get("notes"); !samePath(v.Path, home) {
		t.Errorf("notes was overwritten: %+v", v)
	}
	// An explicit name wins over the fallback.
	if n, err := c.Register("work", "notes", work, KindAgent); err != nil || n != "work" || c.Agent != "work" {
		t.Errorf("explicit = %q, %v (agent %q)", n, err, c.Agent)
	}
	// Renaming a registered folder through init is refused: one folder, one name.
	if _, err := c.Register("job", "", work, KindAgent); err == nil {
		t.Error("a second name for a registered folder should fail")
	}
	if _, err := c.Register("", "", filepath.Join(root, "x"), KindPersonal); err == nil {
		t.Error("no name at all should fail")
	}
}
