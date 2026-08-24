"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";
import type { OrderStatus } from "@/lib/status";

type Task = {
  id: string;
  order_number: string;
  status: OrderStatus;
  pickup_zone_id: number;
};

type Availability = "AVAILABLE" | "BUSY" | "OFFLINE";

export default function AgentTaskListPage() {
  const [availability, setAvailability] = useState<Availability>("AVAILABLE");
  const [tasks, setTasks] = useState<Task[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setTasks(await apiFetch<Task[]>("/agents/me/orders"));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load tasks");
    }
  }, []);

  useEffect(() => {
    // fetch-on-mount + 15s polling; state updates happen after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  async function changeAvailability(value: Availability) {
    setAvailability(value);
    try {
      await apiFetch("/agents/me/availability", {
        method: "PATCH",
        body: { availability: value },
      });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update availability");
    }
  }

  return (
    <main className="mx-auto w-full max-w-md flex-1 p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">My Deliveries</h1>
        <select
          value={availability}
          onChange={(e) => changeAvailability(e.target.value as Availability)}
          className="rounded-md border border-zinc-300 px-2 py-1.5 text-sm bg-white"
        >
          <option value="AVAILABLE">Available</option>
          <option value="BUSY">Busy</option>
          <option value="OFFLINE">Offline</option>
        </select>
      </div>

      <div className="rounded-lg border border-zinc-200 bg-white shadow-sm divide-y divide-zinc-100">
        {error && <p className="p-4 text-sm text-red-600">{error}</p>}
        {!error && tasks.length === 0 && (
          <p className="p-4 text-sm text-zinc-400">No assigned deliveries.</p>
        )}
        {tasks.map((t) => (
          <Link
            key={t.id}
            href={`/agent/orders/${t.id}`}
            className="flex items-center justify-between px-4 py-3 hover:bg-zinc-50"
          >
            <span className="font-mono text-sm font-medium">#{t.order_number}</span>
            <span className="text-xs text-zinc-500">Zone {t.pickup_zone_id}</span>
            <StatusChip status={t.status} />
          </Link>
        ))}
      </div>

      <p className="mt-4 text-xs text-zinc-400">
        Setting yourself Offline removes you from auto-assignment.
      </p>
    </main>
  );
}

function StatusChip({ status }: { status: OrderStatus }) {
  const tone =
    status === "DELIVERED"
      ? "bg-green-100 text-green-800"
      : status === "FAILED"
        ? "bg-red-100 text-red-700"
        : "bg-zinc-100 text-zinc-700";
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${tone}`}>
      {status.replace(/_/g, " ").toLowerCase()}
    </span>
  );
}
