"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";
import { formatINR } from "@/lib/status";

type Zone = { id: number; name: string; description?: string; area_count: number };
type Lane = {
  from_zone_id: number;
  to_zone_id: number;
  from_zone_name?: string;
  to_zone_name?: string;
  base_price: number;
  additional_price_per_kg: number;
};
type RateCard = {
  id: number;
  order_type: string;
  is_active: boolean;
  lanes: Lane[];
};

export default function ZonesPage() {
  const [orderType, setOrderType] = useState<"B2B" | "B2C">("B2C");
  const [zones, setZones] = useState<Zone[]>([]);
  const [rateCard, setRateCard] = useState<RateCard | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<Lane | null>(null);

  async function load() {
    try {
      setZones(await apiFetch<Zone[]>("/admin/zones"));
      const cards = await apiFetch<RateCard[]>("/admin/rate-cards");
      setRateCard(cards.find((c) => c.order_type === orderType) ?? null);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load");
    }
  }

  useEffect(() => {
    // fetch-on-mount; state updates happen after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orderType]);

  async function addZone() {
    const name = window.prompt("Zone name (e.g. Zone A — Local)");
    if (!name) return;
    try {
      await apiFetch("/admin/zones", { method: "POST", body: { name, pincodes: [] } });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Create failed");
    }
  }

  async function saveLane() {
    if (!editing) return;
    try {
      await apiFetch(`/admin/rate-cards/${rateCard?.id}/lanes`, {
        method: "PATCH",
        body: editing,
      });
      setEditing(null);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Save failed");
    }
  }

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 p-6 space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Zones & rate cards</h1>
        <Link href="/admin" className="text-sm text-blue-600 hover:underline">← Orders</Link>
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <section>
        <div className="flex items-center justify-between mb-2">
          <h2 className="font-medium">Zones</h2>
          <button onClick={addZone} className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50">
            + Add Zone
          </button>
        </div>
        <ul className="rounded-lg border border-zinc-200 bg-white shadow-sm divide-y divide-zinc-100 text-sm">
          {zones.map((z) => (
            <li key={z.id} className="flex justify-between px-4 py-3">
              <span>{z.name}</span>
              <span className="text-zinc-400">{z.area_count} areas</span>
            </li>
          ))}
          {zones.length === 0 && <li className="px-4 py-3 text-zinc-400">No zones yet.</li>}
        </ul>
      </section>

      <section>
        <div className="flex items-center gap-3 mb-2">
          <h2 className="font-medium">Rate Cards</h2>
          <select
            value={orderType}
            onChange={(e) => setOrderType(e.target.value as "B2B" | "B2C")}
            className="rounded-md border border-zinc-300 px-2 py-1 text-sm bg-white"
          >
            <option value="B2C">B2C</option>
            <option value="B2B">B2B</option>
          </select>
          {rateCard?.is_active && (
            <span className="rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">Active</span>
          )}
        </div>
        <table className="w-full rounded-lg border border-zinc-200 bg-white shadow-sm text-sm">
          <thead>
            <tr className="border-b border-zinc-200 text-left text-xs uppercase tracking-wide text-zinc-500">
              <th className="px-4 py-2">From → To</th>
              <th className="px-4 py-2">Base</th>
              <th className="px-4 py-2">+/kg</th>
              <th className="px-4 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-100">
            {(rateCard?.lanes ?? []).map((lane) => (
              <tr key={`${lane.from_zone_id}-${lane.to_zone_id}`}>
                <td className="px-4 py-2 font-mono text-xs">
                  {lane.from_zone_name ?? lane.from_zone_id} → {lane.to_zone_name ?? lane.to_zone_id}
                </td>
                <td className="px-4 py-2">{formatINR(lane.base_price)}</td>
                <td className="px-4 py-2">{formatINR(lane.additional_price_per_kg)}</td>
                <td className="px-4 py-2 text-right">
                  <button
                    onClick={() => setEditing(lane)}
                    className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50"
                  >
                    Edit Row
                  </button>
                </td>
              </tr>
            ))}
            {!rateCard && (
              <tr><td colSpan={4} className="px-4 py-3 text-zinc-400">No active card for {orderType}.</td></tr>
            )}
          </tbody>
        </table>
      </section>

      {editing && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4">
          <form
            onSubmit={(e) => { e.preventDefault(); saveLane(); }}
            className="w-full max-w-sm rounded-lg bg-white p-6 shadow-lg space-y-4"
          >
            <h3 className="font-semibold">Edit lane pricing</h3>
            <label className="block text-sm">
              Base price (₹)
              <input
                type="number"
                step="0.01"
                value={editing.base_price}
                onChange={(e) => setEditing({ ...editing, base_price: Number(e.target.value) })}
                className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2"
              />
            </label>
            <label className="block text-sm">
              Additional per kg (₹)
              <input
                type="number"
                step="0.01"
                value={editing.additional_price_per_kg}
                onChange={(e) => setEditing({ ...editing, additional_price_per_kg: Number(e.target.value) })}
                className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2"
              />
            </label>
            <div className="flex gap-2">
              <button type="submit" className="flex-1 rounded-md bg-black py-2 text-white text-sm hover:bg-zinc-800">Save</button>
              <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-md border border-zinc-300 py-2 text-sm">Cancel</button>
            </div>
          </form>
        </div>
      )}
    </main>
  );
}
