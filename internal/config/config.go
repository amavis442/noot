// Package config stores the named vaults in a small YAML file:
// %AppData%\noot\config.yaml on Windows, ~/.config/noot/config.yaml on Linux
// (override with $NOOT_CONFIG).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v3"
)

// Kind is the purpose of a vault; it decides the layout created by init.
type Kind string

const (
	KindPersonal Kind = "personal" // your own notes: journal, zettels, projects
	KindAgent    Kind = "agent"    // written by an AI agent: AGENTS.md, logs, memory, decisions
)

// ParseKind validates a kind string ("" means personal).
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case "", KindPersonal:
		return KindPersonal, nil
	case KindAgent:
		return KindAgent, nil
	}
	return "", fmt.Errorf("unknown vault kind %q (use personal or agent)", s)
}

// Vault is a registered vault.
type Vault struct {
	Path string `yaml:"path"`
	Kind Kind   `yaml:"kind,omitempty"`
}

// Config is the file content.
type Config struct {
	Default string           `yaml:"default,omitempty"` // used when no --vault is given and cwd is not inside a vault
	Agent   string           `yaml:"agent,omitempty"`   // used by --agent / -A
	Vaults  map[string]Vault `yaml:"vaults,omitempty"`

	path string
}

// File returns the config file location.
func File() (string, error) {
	if p := os.Getenv("NOOT_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "noot", "config.yaml"), nil
}

// Load reads the config; a missing file yields an empty config.
func Load() (*Config, error) {
	p, err := File()
	if err != nil {
		return nil, err
	}
	c := &Config{path: p, Vaults: map[string]Vault{}}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if c.Vaults == nil {
		c.Vaults = map[string]Vault{}
	}
	return c, nil
}

// Save writes the config, creating its directory.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0o644)
}

// Path is where this config lives.
func (c *Config) Path() string { return c.path }

// Add registers (or updates) a vault. The first personal vault becomes the
// default and the first agent vault becomes the agent vault.
func (c *Config) Add(name, path string, kind Kind) error {
	if name == "" {
		return errors.New("vault name is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	c.Vaults[name] = Vault{Path: abs, Kind: kind}
	switch {
	case kind == KindAgent && c.Agent == "":
		c.Agent = name
	case kind != KindAgent && c.Default == "":
		c.Default = name
	}
	return nil
}

// Remove unregisters a vault (files are untouched).
func (c *Config) Remove(name string) error {
	if _, ok := c.Vaults[name]; !ok {
		return fmt.Errorf("no vault named %q", name)
	}
	delete(c.Vaults, name)
	if c.Default == name {
		c.Default = ""
	}
	if c.Agent == name {
		c.Agent = ""
	}
	return nil
}

// Get looks up a vault by name.
func (c *Config) Get(name string) (Vault, bool) {
	v, ok := c.Vaults[name]
	return v, ok
}

// Names returns all vault names, sorted.
func (c *Config) Names() []string {
	out := make([]string, 0, len(c.Vaults))
	for n := range c.Vaults {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// NameFor returns the registered name of a path, if any.
func (c *Config) NameFor(path string) string {
	for n, v := range c.Vaults {
		if samePath(v.Path, path) {
			return n
		}
	}
	return ""
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if a == b {
		return true
	}
	// Windows paths are case-insensitive.
	return filepath.Separator == '\\' && equalFold(a, b)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
