import { useEffect, useState } from "react";

export type Route =
  | { name: "home" }
  | { name: "new" }
  | { name: "note"; ref: string }
  | { name: "journal"; day: string }
  | { name: "agenda"; day: string }
  | { name: "reminders" };

const notePrefix = "#/note/";
const journalPrefix = "#/journal/";
const agendaPrefix = "#/agenda/";

/** Reads a route from location.hash; anything unknown is home. */
export function parseRoute(hash: string): Route {
  if (hash === "#/new") {
    return { name: "new" };
  }
  if (hash === "#/reminders") {
    return { name: "reminders" };
  }
  if (hash === "#/journal") {
    return { name: "journal", day: "today" };
  }
  if (hash.startsWith(journalPrefix)) {
    return { name: "journal", day: hash.slice(journalPrefix.length) };
  }
  if (hash === "#/agenda") {
    return { name: "agenda", day: "today" };
  }
  if (hash.startsWith(agendaPrefix)) {
    return { name: "agenda", day: hash.slice(agendaPrefix.length) };
  }
  if (hash.startsWith(notePrefix)) {
    try {
      return { name: "note", ref: decodeURIComponent(hash.slice(notePrefix.length)) };
    } catch {
      return { name: "home" };
    }
  }
  return { name: "home" };
}

/** The current route, updated when the hash changes. */
export function useHashRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    const onChange = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return route;
}
