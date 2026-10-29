package vault

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestNewIDIsAULID(t *testing.T) {
	id, err := NewID(time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != idLen {
		t.Fatalf("NewID() = %q, want %d characters", id, idLen)
	}
	// Crockford base32 without I, L, O and U, so an id cannot be misread.
	if strings.Trim(id, crockford) != "" {
		t.Errorf("NewID() = %q, contains characters outside %q", id, crockford)
	}
	// The timestamp is 48 bits, so the first character never overflows.
	if id[0] > '7' {
		t.Errorf("NewID() = %q, first character should hold the 3 high bits", id)
	}
}

func TestNewIDIsUniqueWithinOneMillisecond(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := NewID(now)
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %s after %d draws in the same millisecond", id, i)
		}
		seen[id] = true
	}
}

func TestNewIDsSortByTime(t *testing.T) {
	base := time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC)
	var ids []string
	for _, d := range []time.Duration{0, time.Millisecond, time.Second, time.Hour, 24 * time.Hour} {
		id, err := NewID(base.Add(d))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("ids are not in time order:\n%s", strings.Join(ids, "\n"))
	}
}
