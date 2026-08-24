"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";

type Candidate = {
  id: string;
  name: string;
  distance_km: number;
  availability: "AVAILABLE" | "BUSY" | "OFFLINE";
};

type PendingOrder = {
  id: string;
  order_number: string;
  pickup_zone_id: number;
};

export default function AssignmentsPage() {
  const [pending, setPending] = useState<PendingOrder[]>([]);
  const [selected, setSelected] = useState<PendingOrder | null>(null);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function loadPending() {
    try {
      setPending(await apiFetch<PendingOrder[]>("/admin/orders?status=CREATED&status=FAILED"));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load pending orders");
    }
  }

  useEffect(() => {
    // fetch-on-mount; state updates happen after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadPending();
  }, []);

  const pick = useCallback(async (order: PendingOrder) => {
    setSelected(order);
    try {
      setCandidates(await apiFetch<Candidate[]>(`/admin/orders/${order.id}/nearby-agents`));
      setError(null);
    } catch (err) {
      setCandidates([]);
      setError(err instanceof ApiError ? err.message : "Failed to load nearby agents");
    }
  }, []);

  async function assignManual(orderId: string, agentId: string) {
    try {
      await apiFetch(`/orders/${orderId}/assign`, {
        method: "POST",
        body: { mode: "MANUAL", agent_id: agentId },
      });
      setSelected(null);
      await loadPending();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Assignment failed");
    }
  }

  return (
    <main className="mx-auto w-full max-w-md flex-1 p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">Agent assignment</h1>
        <Link href="/admin" className="text-sm text-blue-600 hover:underline">← Orders</Link>
      </div>

      {error && <p className="mb-3 text-sm text-red-600">{error}</p>}

      <div className="rounded-lg border border-zinc-200 bg-white shadow-sm divide-y divide-zinc-100 text-sm">
        {pending.length === 0 && <p className="p-4 text-zinc-400">No orders waiting for an agent.</p>}
        {pending.map((o) => (
          <button
            key={o.id}
            onClick={() => pick(o)}
            disabled={selected?.id === o.id}
            className="flex w-full items-center justify-between px-4 py-3 hover:bg-zinc-50"
          >
            <span className="font-mono font-medium">#{o.order_number}</span>
            <span className="text-xs text-zinc-500">Zone {o.pickup_zone_id} · needs an agent</span>
          </button>
        ))}
      </div>

      {selected && (
        <section className="mt-6 rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
          <h2 className="text-sm font-semibold mb-3">Nearby agents (Zone {selected.pickup_zone_id})</h2>
          <ul className="space-y-2">
            {candidates.map((c) => (
              <li key={c.id} className="flex items-center justify-between text-sm">
                <label className="flex items-center gap-2">
                  <input type="radio" name="agent" value={c.id} onChange={() => assignManual(selected.id, c.id)} />
                  <span>{c.name}</span>
                </label>
                <span className="flex items-center gap-2 text-xs text-zinc-500">
                  {c.distance_km.toFixed(1)} km
                  <span
                    className={`rounded-full px-2 py-0.5 ${
                      c.availability === "AVAILABLE" ? "bg-green-100 text-green-800" : "bg-zinc-100 text-zinc-600"
                    }`}
                  >
                    {c.availability === "AVAILABLE" ? "FREE" : c.availability}
                  </span>
                </span>
              </li>
            ))}
            {candidates.length === 0 && <li className="text-zinc-400">No agents in range.</li>}
          </ul>
          <p className="mt-3 text-xs text-zinc-400">
            Ranked by the same Haversine distance the auto-assignment engine uses.
          </p>
        </section>
      )}
    </main>
  );
}
