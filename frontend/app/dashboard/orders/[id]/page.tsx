"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import { StatusTimeline } from "@/components/StatusTimeline";
import type { OrderStatus } from "@/lib/status";

type OrderDetail = {
  id: string;
  order_number: string;
  status: OrderStatus;
  assigned_agent_id?: string | null;
  charge_breakdown?: {
    base_charge: number;
    cod_surcharge: number;
    fuel_surcharge: number;
    gst_amount: number;
    total_charge: number;
  };
  status_history?: { status: string; created_at?: string }[];
};

export default function TrackOrderPage() {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<OrderDetail>(`/orders/${id}`)
      .then(setOrder)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load order"));
  }, [id]);

  if (error) {
    return <Main><p className="text-red-600">{error}</p></Main>;
  }
  if (!order) {
    return <Main><p className="text-zinc-400">Loading…</p></Main>;
  }

  return (
    <Main>
      <div className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-lg font-mono font-semibold">{order.order_number || order.id}</h1>
          <Link href="/dashboard" className="text-sm text-blue-600 hover:underline">
            ← New delivery
          </Link>
        </div>

        <StatusTimeline currentStatus={order.status} history={order.status_history ?? []} />

        {order.charge_breakdown && (
          <dl className="mt-8 pt-4 border-t border-zinc-200 text-sm space-y-1">
            <Row label="Total paid" value={`₹ ${order.charge_breakdown.total_charge.toFixed(2)}`} />
            <Row label="Payment" value="COD" />
          </dl>
        )}

        {order.status === "FAILED" && (
          <Link
            href={`/dashboard/orders/${id}/reschedule`}
            className="mt-6 block w-full rounded-md bg-black py-2.5 text-center text-white text-sm font-medium hover:bg-zinc-800"
          >
            Reschedule delivery
          </Link>
        )}
      </div>
    </Main>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between">
      <dt className="text-zinc-500">{label}</dt>
      <dd className="font-medium">{value}</dd>
    </div>
  );
}

function Main({ children }: { children: React.ReactNode }) {
  return <main className="mx-auto w-full max-w-md flex-1 p-6">{children}</main>;
}
