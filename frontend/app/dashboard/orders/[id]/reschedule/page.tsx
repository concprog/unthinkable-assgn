"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";

export default function ReschedulePage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [date, setDate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await apiFetch(`/orders/${id}/reschedule`, {
        method: "POST",
        body: { requested_date: date },
      });
      router.push(`/dashboard/orders/${id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto w-full max-w-md flex-1 p-6">
      <form onSubmit={submit} className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm space-y-4">
        <h1 className="text-lg font-semibold">Reschedule delivery</h1>
        <p className="text-sm text-zinc-500">
          The previous attempt failed. Pick a new date — a new agent will be assigned automatically.
        </p>
        <div className="flex flex-col gap-1">
          <label htmlFor="requested_date" className="text-xs font-medium text-zinc-600 uppercase tracking-wide">
            New date
          </label>
          <input
            id="requested_date"
            type="date"
            required
            value={date}
            onChange={(e) => setDate(e.target.value)}
            className="rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-black focus:outline-none"
          />
        </div>
        <button
          type="submit"
          disabled={busy || !date}
          className="w-full rounded-md bg-black py-2.5 text-white text-sm font-medium hover:bg-zinc-800 disabled:opacity-50"
        >
          {busy ? "Submitting…" : "Submit"}
        </button>
        {error && <p className="text-sm text-red-600">{error}</p>}
      </form>
    </main>
  );
}
