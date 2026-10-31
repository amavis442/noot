import { ReminderForm } from "./ReminderForm";
import { ReminderItem } from "./ReminderItem";
import { useReminders } from "./useReminders";

export function RemindersPage() {
  const { reminders, error, showDone, setShowDone, add, complete, plan } = useReminders();

  return (
    <section className="reminders">
      <h2>Reminders</h2>
      <ReminderForm onAdd={add} />
      <label className="check">
        <input type="checkbox" checked={showDone} onChange={(e) => setShowDone(e.target.checked)} />
        Toon afgeronde
      </label>
      {error && <p role="alert" className="error">{error}</p>}
      {reminders === null && !error && <p className="muted">Laden…</p>}
      {reminders?.length === 0 && <p className="muted">{showDone ? "Geen reminders." : "Geen open reminders."}</p>}
      {reminders && reminders.length > 0 && (
        <ul className="reminder-list">
          {reminders.map((r) => (
            <ReminderItem key={r.n} reminder={r} onComplete={(n) => void complete(n)} onPlan={plan} />
          ))}
        </ul>
      )}
    </section>
  );
}
