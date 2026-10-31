import { useState } from "react";
import { noteHref } from "../../shared/api";
import { Snippet } from "./Snippet";
import { useSearch } from "./useSearch";

export function SearchPanel() {
  const [query, setQuery] = useState("");
  const { hits, error, loading } = useSearch(query);

  return (
    <section className="search">
      <input
        type="search"
        aria-label="Zoek notities"
        placeholder="Zoek notities…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        autoFocus
      />
      {error && <p role="alert" className="error">{error}</p>}
      {loading && <p className="muted">Zoeken…</p>}
      {!loading && hits?.length === 0 && <p className="muted">Geen notities gevonden.</p>}
      {hits && hits.length > 0 && (
        <ul className="hits">
          {hits.map((hit) => (
            <li key={hit.id}>
              <a href={noteHref(hit.path)}>
                <strong>{hit.title}</strong>
                <span className="path">{hit.path}</span>
              </a>
              {hit.snippet && (
                <p className="snippet">
                  <Snippet text={hit.snippet} />
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
