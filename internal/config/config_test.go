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
