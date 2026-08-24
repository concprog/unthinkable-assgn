"use client";

import { useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";
import { ChargeBreakdownCard, type ChargeBreakdownData } from "@/components/ChargeBreakdown";

type CreateResponse = ChargeBreakdownData & { order_id: string; status: string };

export default function NewOrderPage() {
  const [form, setForm] = useState({
    pickup_pincode: "",
    drop_pincode: "",
    pickup_line1: "",
    drop_line1: "",
    pickup_city: "",
    drop_city: "",
    length_cm: "",
    breadth_cm: "",
    height_cm: "",
    actual_weight_kg: "",
    order_type: "B2C",
    payment_type: "COD",
    order_value: "",
  });
  const [preview, setPreview] = useState<CreateResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function set<K extends keyof typeof form>(key: K, value: string) {
    setForm((f) => ({ ...f, [key]: value }));
    setPreview(null);
    setError(null);
  }

  async function calculate(e?: React.FormEvent) {
    e?.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await apiFetch<CreateResponse>("/orders", {
        method: "POST",
        body: {
          pickup_address: {
            line1: form.pickup_line1,
            city: form.pickup_city,
            pincode: form.pickup_pincode,
          },
          drop_address: {
            line1: form.drop_line1,
            city: form.drop_city,
            pincode: form.drop_pincode,
          },
          package: {
            length_cm: Number(form.length_cm),
            breadth_cm: Number(form.breadth_cm),
            height_cm: Number(form.height_cm),
            actual_weight_kg: Number(form.actual_weight_kg),
          },
          order_type: form.order_type,
          payment_type: form.payment_type,
          order_value: Number(form.order_value),
        },
      });
      setPreview(res);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto w-full max-w-md flex-1 p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">New Delivery</h1>
        <Link href="/dashboard/orders" className="text-sm text-blue-600 hover:underline">
          My orders →
        </Link>
      </div>

      <form onSubmit={calculate} className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm space-y-4">
        <Field label="Pickup address" id="pickup_line1" value={form.pickup_line1} onChange={(v) => set("pickup_line1", v)} placeholder="12 MG Road" />
        <div className="grid grid-cols-2 gap-3">
          <Field label="City" id="pickup_city" value={form.pickup_city} onChange={(v) => set("pickup_city", v)} placeholder="Chennai" />
          <Field label="Pincode" id="pickup_pincode" value={form.pickup_pincode} onChange={(v) => set("pickup_pincode", v)} placeholder="600001" />
        </div>
        <hr />
        <Field label="Drop address" id="drop_line1" value={form.drop_line1} onChange={(v) => set("drop_line1", v)} placeholder="45 Anna Nagar" />
        <div className="grid grid-cols-2 gap-3">
          <Field label="City" id="drop_city" value={form.drop_city} onChange={(v) => set("drop_city", v)} />
          <Field label="Pincode" id="drop_pincode" value={form.drop_pincode} onChange={(v) => set("drop_pincode", v)} />
        </div>
        <hr />
        <div className="grid grid-cols-3 gap-3">
          <Field label="L (cm)" id="length_cm" value={form.length_cm} onChange={(v) => set("length_cm", v)} type="number" step="0.1" />
          <Field label="B (cm)" id="breadth_cm" value={form.breadth_cm} onChange={(v) => set("breadth_cm", v)} type="number" step="0.1" />
          <Field label="H (cm)" id="height_cm" value={form.height_cm} onChange={(v) => set("height_cm", v)} type="number" step="0.1" />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Weight (kg)" id="actual_weight_kg" value={form.actual_weight_kg} onChange={(v) => set("actual_weight_kg", v)} type="number" step="0.01" />
          <Field label="Order value (₹)" id="order_value" value={form.order_value} onChange={(v) => set("order_value", v)} type="number" step="0.01" />
        </div>

        <Radio
          legend="Type"
          name="order_type"
          options={["B2B", "B2C"]}
          value={form.order_type}
          onChange={(v) => set("order_type", v)}
        />
        <Radio
          legend="Payment"
          name="payment_type"
          options={["PREPAID", "COD"]}
          labels={{ PREPAID: "Prepaid", COD: "COD" }}
          value={form.payment_type}
          onChange={(v) => set("payment_type", v)}
        />

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-md bg-black py-2.5 text-white text-sm font-medium hover:bg-zinc-800 disabled:opacity-50"
        >
          {busy ? "Calculating…" : "Calculate Charge"}
        </button>
        {error && <p className="text-sm text-red-600">{error}</p>}
      </form>

      {preview && (
        <ChargeBreakdownCard data={preview} onConfirm={() => setPreview(null)} confirming={false} />
      )}
    </main>
  );
}

function Field({
  label,
  id,
  value,
  onChange,
  type = "text",
  placeholder,
  step,
}: {
  label: string;
  id: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  step?: string;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs font-medium text-zinc-600 uppercase tracking-wide">
        {label}
      </label>
      <input
        id={id}
        type={type}
        step={step}
        inputMode={type === "number" ? "decimal" : undefined}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        required
        className="rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-black focus:outline-none"
      />
    </div>
  );
}

function Radio({
  legend,
  name,
  options,
  labels = {},
  value,
  onChange,
}: {
  legend: string;
  name: string;
  options: string[];
  labels?: Record<string, string>;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <fieldset className="flex gap-6">
      <legend className="sr-only">{legend}</legend>
      <span className="text-xs font-medium text-zinc-600 uppercase tracking-wide pt-1">{legend}</span>
      {options.map((opt) => (
        <label key={opt} className="flex items-center gap-1.5 text-sm cursor-pointer">
          <input
            type="radio"
            name={name}
            checked={value === opt}
            onChange={() => onChange(opt)}
            className="accent-black"
          />
          {labels[opt] ?? opt}
        </label>
      ))}
    </fieldset>
  );
}
