import { useState, type FormEvent } from "react";
import type { Reminder } from "../../shared/api";

interface Props {
  reminder: Reminder;
  onComplete: (n: number) => void;
  onPlan: (n: number, day: string) => Promise<boolean>;
}

/** One reminder: a checkbox to mark it done, its dates and a way to plan it. */
export function ReminderItem({ reminder: r, onComplete, onPlan }: Props) {
  const [day, setDay] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (await onPlan(r.n, day.trim())) {
      setDay("");
    }
  }

  const meta = [
    r.due && `uiterlijk ${r.due}`,
    r.planned && `gepland ${r.planned}`,
    r.done_on && `afgerond ${r.done_on}`,
  ].filter(Boolean);

  return (
    <li className={r.done ? "reminder done" : "reminder"}>
      <label className="check">
        <input type="checkbox" checked={r.done} disabled={r.done} onChange={() => onComplete(r.n)} aria-label={r.text} />
        <span>{r.text}</span>
      </label>
      {meta.length > 0 && <p className="path">{meta.join(" · ")}</p>}
      {!r.done && (
        <form className="plan" onSubmit={onSubmit}>
          <input aria-label="Inplannen op" placeholder="dag, bv. woensdag" value={day} onChange={(e) => setDay(e.target.value)} />
          <button type="submit" className="secondary" disabled={day.trim() === ""}>
            Inplannen
          </button>
        </form>
      )}
    </li>
  );
}
