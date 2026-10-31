/** The day n days after date (YYYY-MM-DD); n may be negative. */
export function shiftDay(date: string, n: number): string {
  // UTC, so a change to or from daylight saving time cannot skip or repeat a day.
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

/** The frontend route that shows the journal of day. */
export function journalHref(day: string): string {
  return `#/journal/${day}`;
}
