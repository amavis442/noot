import { useState, type FormEvent } from "react";

/** Adds a reminder; onAdd resolves to whether it worked, which clears the form. */
export function ReminderForm({ onAdd }: { onAdd: (text: string, due: string) => Promise<boolean> }) {
  const [text, setText] = useState("");
  const [due, setDue] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (await onAdd(text.trim(), due)) {
      setText("");
      setDue("");
    }
  }

  return (
    <form className="reminder-form" onSubmit={onSubmit}>
      <label>
        Nieuwe reminder
        <input value={text} onChange={(e) => setText(e.target.value)} required />
      </label>
      <label>
        Uiterlijk (optioneel)
        <input value={due} onChange={(e) => setDue(e.target.value)} placeholder="morgen, vrijdag, 2026-11-06" />
      </label>
      <button type="submit" disabled={text.trim() === ""}>
        Toevoegen
      </button>
    </form>
  );
}
