package kit

import (
	"fmt"
	"time"

	"github.com/amavis442/noot/internal/vault"
)

// dutchMonths are indexed by time.Month - 1.
var dutchMonths = [...]string{"januari", "februari", "maart", "april", "mei", "juni", "juli", "augustus", "september", "oktober", "november", "december"}

// DutchDate formats day as "zaterdag 31 oktober 2026".
func DutchDate(day time.Time) string {
	return fmt.Sprintf("%s %s %d", vault.DutchWeekdays[day.Weekday()], DutchDayMonth(day), day.Year())
}

// DutchDayMonth formats day as "31 oktober".
func DutchDayMonth(day time.Time) string {
	return fmt.Sprintf("%d %s", day.Day(), dutchMonths[day.Month()-1])
}

// SameDay reports whether a and b fall on the same calendar day.
func SameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Midnight returns the start of t's day, in t's location.
func Midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
