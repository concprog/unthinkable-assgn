"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";

type AdminOrder = {
  id: string;
  order_number: string;
  order_type: string;
  status: string;
  pickup_zone_id: number;
  assigned_agent_name?: string | null;
};

export default function AdminOrdersPage() {
  const [orders, setOrders] = useState<AdminOrder[]>([]);
  const [filters, setFilters] = useState({ zone: "", status: "", agent: "" });
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const qs = new URLSearchParams();
    if (filters.zone) qs.set("zone", filters.zone);
    if (filters.status) qs.set("status", filters.status);
    if (filters.agent) qs.set("agent", filters.agent);
    try {
      setOrders(await apiFetch<AdminOrder[]>(`/admin/orders?${qs.toString()}`));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load orders");
    }
  }, [filters]);

  useEffect(() => {
    // fetch-on-mount; state updates happen after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  async function overrideStatus(order: AdminOrder) {
    const next = window.prompt(`Override #${order.order_number} (${order.status}) to:`);
    if (!next) return;
    try {
      await apiFetch(`/orders/${order.id}/status`, {
        method: "PATCH",
        body: { status: next.toUpperCase(), actor_type: "ADMIN" },
      });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Override failed");
    }
  }

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">Orders</h1>
        <nav className="flex gap-4 text-sm">
          <Link href="/admin/zones" className="text-blue-600 hover:underline">Zones & rates</Link>
          <Link href="/admin/assignments" className="text-blue-600 hover:underline">Assignments</Link>
        </nav>
      </div>

      <div className="flex gap-3 mb-4 text-sm">
        <select
          value={filters.zone}
          onChange={(e) => setFilters((f) => ({ ...f, zone: e.target.value }))}
          className="rounded-md border border-zinc-300 px-2 py-1.5 bg-white"
        >
          <option value="">All zones</option>
          <option value="A">Zone A</option>
          <option value="B">Zone B</option>
        </select>
        <select
          value={filters.status}
          onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))}
          className="rounded-md border border-zinc-300 px-2 py-1.5 bg-white"
        >
          <option value="">All statuses</option>
          {["CREATED","CONFIRMED","ASSIGNED","PICKED_UP","IN_TRANSIT","OUT_FOR_DELIVERY","DELIVERED","FAILED","RESCHEDULED","CANCELLED"].map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
      </div>

      <div className="rounded-lg border border-zinc-200 bg-white shadow-sm divide-y divide-zinc-100">
        {error && <p className="p-4 text-sm text-red-600">{error}</p>}
        {!error && orders.length === 0 && <p className="p-4 text-sm text-zinc-400">No orders match.</p>}
        {orders.map((o) => (
          <div key={o.id} className="flex items-center gap-4 px-4 py-3 text-sm">
            <span className="font-mono font-medium w-16">#{o.order_number}</span>
            <span className="w-12 text-zinc-500">{o.order_type}</span>
            <span className="w-32">{o.status.replace(/_/g, " ").toLowerCase()}</span>
            <span className="w-24 text-zinc-500">{o.assigned_agent_name ?? "---"}</span>
            <button
              onClick={() => overrideStatus(o)}
              className="ml-auto rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50"
            >
              Override Status
            </button>
          </div>
        ))}
      </div>
      <p className="mt-3 text-xs text-zinc-400">
        Overrides skip the state machine but are always logged to order_status_history as ADMIN.
      </p>
    </main>
  );
}
