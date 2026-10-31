/** The day n days after date (YYYY-MM-DD); n may be negative. */
export function shiftDay(date: string, n: number): string {
  // UTC, so a change to or from daylight saving time cannot skip or repeat a day.
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

const weekdays = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

/** The English name of the weekday of date (YYYY-MM-DD), e.g. "Monday". */
export function weekdayName(date: string): string {
  // UTC, as date is parsed as midnight UTC: getDay() in a time zone west of
  // UTC would give the day before.
  return weekdays[new Date(`${date}T00:00:00Z`).getUTCDay()];
}

/** The frontend route that shows the journal of day. */
export function journalHref(day: string): string {
  return `#/journal/${day}`;
}

/** The frontend route that shows the agenda of the week day falls in. */
export function agendaHref(day: string): string {
  return `#/agenda/${day}`;
}
