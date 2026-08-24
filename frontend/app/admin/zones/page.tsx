"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";
import { Modal } from "@/components/Modal";
import { formatINR } from "@/lib/status";

type Zone = { id: number; name: string; description?: string; area_count: number };
type Area = { pincode: string; city?: string | null; state?: string | null };
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
  name?: string;
  volumetric_divisor: number;
  cod_surcharge_flat: number;
  cod_surcharge_pct: number;
  fuel_surcharge_pct: number;
  gst_pct: number;
  is_active: boolean;
  lanes: Lane[];
};

export default function ZonesPage() {
  const [orderType, setOrderType] = useState<"B2B" | "B2C">("B2C");
  const [zones, setZones] = useState<Zone[]>([]);
  const [rateCard, setRateCard] = useState<RateCard | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [flash, setFlash] = useState<string | null>(null);

  const [addZoneOpen, setAddZoneOpen] = useState(false);
  const [areasZone, setAreasZone] = useState<Zone | null>(null);
  const [laneModal, setLaneModal] = useState<"new" | Lane | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      setZones(await apiFetch<Zone[]>("/admin/zones"));
      const cards = await apiFetch<RateCard[]>("/admin/rate-cards");
      const active =
        cards.find((c) => c.order_type === orderType && c.is_active) ??
        cards.find((c) => c.order_type === orderType) ??
        null;
      setRateCard(active);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load");
    }
  }, [orderType]);

  useEffect(() => {
    // fetch-on-mount / on order-type switch; state updates after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  function done(msg: string) {
    setFlash(msg);
    setError(null);
  }

  async function toggleActive() {
    if (!rateCard) return;
    try {
      await apiFetch(`/admin/rate-cards/${rateCard.id}`, {
        method: "PATCH",
        body: { is_active: !rateCard.is_active },
      });
      done(rateCard.is_active ? "Rate card deactivated." : "Rate card activated.");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Toggle failed");
    }
  }

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 space-y-8 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Zones & rate cards</h1>
        <Link href="/admin" className="text-sm text-blue-600 hover:underline">← Orders</Link>
      </div>

      {error && <p className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
      {!error && flash && (
        <p className="rounded-md bg-green-50 px-3 py-2 text-sm text-green-700">{flash}</p>
      )}

      {/* ---------------- ZONES ---------------- */}
      <section>
        <div className="mb-2 flex items-center justify-between">
          <h2 className="font-medium">Zones</h2>
          <button onClick={() => setAddZoneOpen(true)} className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50">
            + Add Zone
          </button>
        </div>
        <ul className="divide-y divide-zinc-100 rounded-lg border border-zinc-200 bg-white text-sm shadow-sm">
          {zones.map((z) => (
            <li key={z.id} className="flex items-center gap-4 px-4 py-3">
              <span>{z.name}</span>
              <span className="text-zinc-400">{z.area_count} areas</span>
              <button
                onClick={() => setAreasZone(z)}
                className="ml-auto rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50"
              >
                Assign areas
              </button>
            </li>
          ))}
          {zones.length === 0 && (
            <li className="px-4 py-3 text-zinc-400">No zones yet — create one to get started.</li>
          )}
        </ul>
      </section>

      {/* ---------------- RATE CARDS ---------------- */}
      <section>
        <div className="mb-2 flex flex-wrap items-center gap-3">
          <h2 className="font-medium">Rate Cards</h2>
          <select
            value={orderType}
            onChange={(e) => {
              setOrderType(e.target.value as "B2B" | "B2C");
              setFlash(null);
            }}
            className="rounded-md border border-zinc-300 bg-white px-2 py-1 text-sm"
          >
            <option value="B2C">B2C</option>
            <option value="B2B">B2B</option>
          </select>
          {rateCard?.is_active ? (
            <span className="rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">Active</span>
          ) : (
            <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-600">Inactive</span>
          )}
          <span className="ml-auto flex gap-2">
            {rateCard && (
              <>
                <button onClick={() => setSettingsOpen(true)} className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50">
                  Settings
                </button>
                <button onClick={toggleActive} className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50">
                  {rateCard.is_active ? "Deactivate" : "Activate"}
                </button>
              </>
            )}
          </span>
        </div>

        {!rateCard ? (
          <div className="flex flex-col items-start gap-3 rounded-lg border border-dashed border-zinc-300 bg-white p-6">
            <p className="text-sm text-zinc-500">No rate card for {orderType} yet.</p>
            {zones.length === 0 ? (
              <p className="text-xs text-zinc-400">Create at least one zone first — lanes connect zones.</p>
            ) : (
              <CreateCardButton orderType={orderType} onDone={(m) => { done(m); void load(); }} />
            )}
          </div>
        ) : (
          <>
            <p className="mb-1 text-xs text-zinc-500">
              {rateCard.name} · volumetric divisor {rateCard.volumetric_divisor} · GST {rateCard.gst_pct}% · fuel{" "}
              {rateCard.fuel_surcharge_pct}% · COD ₹{rateCard.cod_surcharge_flat} + {rateCard.cod_surcharge_pct}%
            </p>
            <table className="w-full rounded-lg border border-zinc-200 bg-white text-sm shadow-sm">
              <thead>
                <tr className="border-b border-zinc-200 text-left text-xs uppercase tracking-wide text-zinc-500">
                  <th className="px-4 py-2">From → To</th>
                  <th className="px-4 py-2">Base</th>
                  <th className="px-4 py-2">+/kg</th>
                  <th className="px-4 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {rateCard.lanes.map((lane) => (
                  <tr key={`${lane.from_zone_id}-${lane.to_zone_id}`}>
                    <td className="px-4 py-2 font-mono text-xs">
                      {lane.from_zone_name ?? lane.from_zone_id} → {lane.to_zone_name ?? lane.to_zone_id}
                    </td>
                    <td className="px-4 py-2">{formatINR(lane.base_price)}</td>
                    <td className="px-4 py-2">{formatINR(lane.additional_price_per_kg)}</td>
                    <td className="px-4 py-2 text-right">
                      <button
                        onClick={() => setLaneModal(lane)}
                        className="rounded-md border border-zinc-300 px-3 py-1 text-xs hover:bg-zinc-50"
                      >
                        Edit
                      </button>
                    </td>
                  </tr>
                ))}
                {rateCard.lanes.length === 0 && (
                  <tr><td colSpan={4} className="px-4 py-3 text-zinc-400">No lanes priced yet.</td></tr>
                )}
              </tbody>
            </table>
            <button
              onClick={() => setLaneModal("new")}
              className="mt-3 rounded-md border border-zinc-300 px-3 py-1.5 text-xs hover:bg-zinc-50"
            >
              + Add lane
            </button>
          </>
        )}
      </section>

      {/* ---------------- MODALS ---------------- */}
      {addZoneOpen && (
        <AddZoneModal onClose={() => setAddZoneOpen(false)} onSaved={(m) => { setAddZoneOpen(false); done(m); void load(); }} />
      )}
      {areasZone && (
        <AreasModal zone={areasZone} onClose={() => setAreasZone(null)} onChanged={(m) => { done(m); void load(); }} />
      )}
      {laneModal && rateCard && (
        <LaneModal
          cardId={rateCard.id}
          zones={zones}
          lane={laneModal === "new" ? null : laneModal}
          onClose={() => setLaneModal(null)}
          onSaved={(m) => { setLaneModal(null); done(m); void load(); }}
        />
      )}
      {settingsOpen && rateCard && (
        <CardSettingsModal card={rateCard} onClose={() => setSettingsOpen(false)} onSaved={(m) => { setSettingsOpen(false); done(m); void load(); }} />
      )}
    </main>
  );
}

function CreateCardButton({ orderType, onDone }: { orderType: "B2B" | "B2C"; onDone: (msg: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)} className="rounded-md bg-black px-3 py-1.5 text-xs font-medium text-white hover:bg-zinc-800">
        Create {orderType} rate card
      </button>
      {open && (
        <CreateCardModal orderType={orderType} onClose={() => setOpen(false)} onSaved={(m) => { setOpen(false); onDone(m); }} />
      )}
    </>
  );
}

function AddZoneModal({ onClose, onSaved }: { onClose: () => void; onSaved: (msg: string) => void }) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [pincodes, setPincodes] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await apiFetch("/admin/zones", {
        method: "POST",
        body: { name, description, pincodes: parsePincodes(pincodes) },
      });
      onSaved(`Zone “${name}” created.`);
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Create failed");
      setBusy(false);
    }
  }

  return (
    <Modal title="Add zone" onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Text label="Name" value={name} onChange={setName} placeholder="Zone A — Local" required />
        <Text label="Description" value={description} onChange={setDescription} placeholder="Same-city shipments" />
        <label className="block text-sm">
          <span className="text-xs font-medium uppercase tracking-wide text-zinc-600">Pincodes (one per line)</span>
          <textarea
            rows={3}
            value={pincodes}
            onChange={(e) => setPincodes(e.target.value)}
            placeholder={"560001 Bangalore KA\n560002 Bangalore KA"}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 font-mono text-sm"
          />
          <span className="mt-1 block text-xs text-zinc-400">Optional — you can also assign areas later.</span>
        </label>
        {err && <p className="text-sm text-red-600">{err}</p>}
        <Actions busy={busy} onCancel={onClose} submitLabel="Create zone" />
      </form>
    </Modal>
  );
}

function AreasModal({ zone, onClose, onChanged }: { zone: Zone; onClose: () => void; onChanged: (msg: string) => void }) {
  const [areas, setAreas] = useState<Area[] | null>(null);
  const [input, setInput] = useState("");
  const [city, setCity] = useState("");
  const [state, setState] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const loadAreas = useCallback(async () => {
    try {
      setAreas(await apiFetch<Area[]>(`/admin/zones/${zone.id}/areas`));
      setErr(null);
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Failed to load areas");
    }
  }, [zone.id]);

  useEffect(() => {
    // fetch-on-open; state updates after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadAreas();
  }, [loadAreas]);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim()) return;
    setBusy(true);
    try {
      await apiFetch(`/admin/zones/${zone.id}/areas`, {
        method: "POST",
        body: { pincodes: [{ pincode: input.trim(), city, state }] },
      });
      setInput("");
      setCity("");
      setState("");
      await loadAreas();
      onChanged(`Area added to ${zone.name}.`);
      setBusy(false);
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Failed to add area");
      setBusy(false);
    }
  }

  async function remove(pincode: string) {
    try {
      await apiFetch(`/admin/zones/${zone.id}/areas/${pincode}`, { method: "DELETE" });
      await loadAreas();
      onChanged("Pincode removed.");
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Failed to remove pincode");
    }
  }

  return (
    <Modal title={`Areas in ${zone.name}`} onClose={onClose}>
      <div className="space-y-4">
        <ul className="max-h-48 space-y-1 overflow-y-auto rounded-md border border-zinc-200 p-2 text-sm">
          {areas?.map((a) => (
            <li key={a.pincode} className="flex items-center justify-between">
              <span className="font-mono text-xs">
                {a.pincode}{a.city ? ` · ${a.city}` : ""}{a.state ? `, ${a.state}` : ""}
              </span>
              <button onClick={() => remove(a.pincode)} className="text-xs text-red-600 hover:underline">
                Remove
              </button>
            </li>
          ))}
          {areas?.length === 0 && <li className="py-2 text-zinc-400">No pincodes assigned yet.</li>}
          {!areas && !err && <li className="py-2 text-zinc-400">Loading…</li>}
        </ul>

        <form onSubmit={add} className="space-y-3 border-t border-zinc-100 pt-4">
          <Text label="Pincode" value={input} onChange={setInput} placeholder="560001" required />
          <div className="flex gap-2">
            <Text label="City (optional)" value={city} onChange={setCity} placeholder="Bangalore" />
            <Text label="State (optional)" value={state} onChange={setState} placeholder="KA" />
          </div>
          <p className="text-xs text-zinc-400">A pincode assigned here is moved out of its previous zone.</p>
          {err && <p className="text-sm text-red-600">{err}</p>}
          <button type="submit" disabled={busy} className="w-full rounded-md bg-black py-2 text-sm text-white hover:bg-zinc-800 disabled:opacity-50">
            {busy ? "…" : "Assign to zone"}
          </button>
        </form>
      </div>
    </Modal>
  );
}

function LaneModal({
  cardId, zones, lane, onClose, onSaved,
}: {
  cardId: number;
  zones: Zone[];
  lane: Lane | null;
  onClose: () => void;
  onSaved: (msg: string) => void;
}) {
  const [fromId, setFromId] = useState(lane?.from_zone_id ?? zones[0]?.id ?? 0);
  const [toId, setToId] = useState(lane?.to_zone_id ?? zones[0]?.id ?? 0);
  const [base, setBase] = useState(String(lane?.base_price ?? ""));
  const [perKg, setPerKg] = useState(String(lane?.additional_price_per_kg ?? ""));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await apiFetch(`/admin/rate-cards/${cardId}/lanes`, {
        method: "PATCH",
        body: {
          from_zone_id: fromId,
          to_zone_id: toId,
          base_price: Number(base),
          additional_price_per_kg: Number(perKg),
        },
      });
      onSaved(lane ? "Lane updated." : "Lane added.");
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Save failed");
      setBusy(false);
    }
  }

  return (
    <Modal title={lane ? "Edit lane pricing" : "Add lane"} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <div className="flex gap-2">
          <label className="block w-1/2 text-sm">
            <span className="text-xs font-medium uppercase tracking-wide text-zinc-600">From zone</span>
            <select value={fromId} onChange={(e) => setFromId(Number(e.target.value))} className="mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm">
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name}</option>)}
            </select>
          </label>
          <label className="block w-1/2 text-sm">
            <span className="text-xs font-medium uppercase tracking-wide text-zinc-600">To zone</span>
            <select value={toId} onChange={(e) => setToId(Number(e.target.value))} className="mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm">
              {zones.map((z) => <option key={z.id} value={z.id}>{z.name}</option>)}
            </select>
          </label>
        </div>
        <p className="text-xs text-zinc-400">Same zone on both sides = intra-zone rate.</p>
        <div className="flex gap-2">
          <Text label="Base price (₹)" value={base} onChange={setBase} type="number" placeholder="40" required />
          <Text label="Additional per kg (₹)" value={perKg} onChange={setPerKg} type="number" placeholder="15" required />
        </div>
        {err && <p className="text-sm text-red-600">{err}</p>}
        <Actions busy={busy} onCancel={onClose} submitLabel={lane ? "Save" : "Add lane"} />
      </form>
    </Modal>
  );
}

function CardSettingsModal({ card, onClose, onSaved }: { card: RateCard; onClose: () => void; onSaved: (msg: string) => void }) {
  const [name, setName] = useState(card.name ?? "");
  const [divisor, setDivisor] = useState(String(card.volumetric_divisor));
  const [codFlat, setCodFlat] = useState(String(card.cod_surcharge_flat));
  const [codPct, setCodPct] = useState(String(card.cod_surcharge_pct));
  const [fuelPct, setFuelPct] = useState(String(card.fuel_surcharge_pct));
  const [gstPct, setGstPct] = useState(String(card.gst_pct));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await apiFetch(`/admin/rate-cards/${card.id}`, {
        method: "PATCH",
        body: {
          name,
          volumetric_divisor: Number(divisor),
          cod_surcharge_flat: Number(codFlat),
          cod_surcharge_pct: Number(codPct),
          fuel_surcharge_pct: Number(fuelPct),
          gst_pct: Number(gstPct),
        },
      });
      onSaved("Rate card settings saved.");
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Save failed");
      setBusy(false);
    }
  }

  return (
    <Modal title={`${card.order_type} card settings`} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Text label="Name" value={name} onChange={setName} placeholder="Standard B2C" required />
        <Text label="Volumetric divisor" value={divisor} onChange={setDivisor} type="number" placeholder="5000" required />
        <div className="flex gap-2">
          <Text label="COD flat (₹)" value={codFlat} onChange={setCodFlat} type="number" placeholder="0" />
          <Text label="COD %" value={codPct} onChange={setCodPct} type="number" placeholder="0" />
        </div>
        <div className="flex gap-2">
          <Text label="Fuel surcharge %" value={fuelPct} onChange={setFuelPct} type="number" placeholder="0" />
          <Text label="GST %" value={gstPct} onChange={setGstPct} type="number" placeholder="18" />
        </div>
        {err && <p className="text-sm text-red-600">{err}</p>}
        <Actions busy={busy} onCancel={onClose} submitLabel="Save settings" />
      </form>
    </Modal>
  );
}

function CreateCardModal({ orderType, onClose, onSaved }: { orderType: "B2B" | "B2C"; onClose: () => void; onSaved: (msg: string) => void }) {
  const [name, setName] = useState(`${orderType} standard`);
  const [divisor, setDivisor] = useState("5000");
  const [codFlat, setCodFlat] = useState("0");
  const [codPct, setCodPct] = useState("0");
  const [fuelPct, setFuelPct] = useState("0");
  const [gstPct, setGstPct] = useState("18");
  const [activate, setActivate] = useState(true);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const { id } = await apiFetch<{ id: number }>("/admin/rate-cards", {
        method: "POST",
        body: {
          order_type: orderType,
          name,
          volumetric_divisor: Number(divisor),
          cod_surcharge_flat: Number(codFlat),
          cod_surcharge_pct: Number(codPct),
          fuel_surcharge_pct: Number(fuelPct),
          gst_pct: Number(gstPct),
        },
      });
      if (activate) {
        await apiFetch(`/admin/rate-cards/${id}`, { method: "PATCH", body: { is_active: true } });
      }
      onSaved(`${orderType} rate card created${activate ? " and activated" : ""}. Now add lanes.`);
    } catch (error) {
      setErr(error instanceof ApiError ? error.message : "Create failed");
      setBusy(false);
    }
  }

  return (
    <Modal title={`New ${orderType} rate card`} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Text label="Name" value={name} onChange={setName} required />
        <Text label="Volumetric divisor" value={divisor} onChange={setDivisor} type="number" required />
        <div className="flex gap-2">
          <Text label="COD flat (₹)" value={codFlat} onChange={setCodFlat} type="number" />
          <Text label="COD %" value={codPct} onChange={setCodPct} type="number" />
        </div>
        <div className="flex gap-2">
          <Text label="Fuel surcharge %" value={fuelPct} onChange={setFuelPct} type="number" />
          <Text label="GST %" value={gstPct} onChange={setGstPct} type="number" />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={activate} onChange={(e) => setActivate(e.target.checked)} />
          Activate immediately (retires any current {orderType} active card)
        </label>
        {err && <p className="text-sm text-red-600">{err}</p>}
        <Actions busy={busy} onCancel={onClose} submitLabel="Create card" />
      </form>
    </Modal>
  );
}

// "pin|city|state" or plain pincode lines → PincodeInput[]
function parsePincodes(raw: string): { pincode: string; city: string; state: string }[] {
  return raw
    .split(/\n+/)
    .map((l) => l.trim())
    .filter(Boolean)
    .flatMap((line) =>
      line.split("|").length > 1
        ? (() => {
            const [p = "", c = "", s = ""] = line.split("|").map((x) => x.trim());
            return p ? [{ pincode: p, city: c, state: s }] : [];
          })()
        : [{ pincode: line, city: "", state: "" }]
    );
}

function Text({
  label, value, onChange, type = "text", placeholder, required,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  required?: boolean;
}) {
  return (
    <label className="block flex-1 text-sm">
      <span className="text-xs font-medium uppercase tracking-wide text-zinc-600">{label}</span>
      <input
        type={type}
        value={value}
        step={type === "number" ? "any" : undefined}
        required={required}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-black focus:outline-none"
      />
    </label>
  );
}

function Actions({ busy, onCancel, submitLabel }: { busy: boolean; onCancel: () => void; submitLabel: string }) {
  return (
    <div className="flex gap-2 pt-1">
      <button type="submit" disabled={busy} className="flex-1 rounded-md bg-black py-2 text-sm text-white hover:bg-zinc-800 disabled:opacity-50">
        {busy ? "…" : submitLabel}
      </button>
      <button type="button" onClick={onCancel} className="flex-1 rounded-md border border-zinc-300 py-2 text-sm">
        Cancel
      </button>
    </div>
  );
}
