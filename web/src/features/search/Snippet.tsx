/** Shows a search snippet with the terms between « and » highlighted. */
export function Snippet({ text }: { text: string }) {
  const parts = text.split(/«|»/);
  // split alternates: outside, inside, outside, ...
  return <>{parts.map((part, i) => (i % 2 === 1 ? <mark key={i}>{part}</mark> : part))}</>;
}
