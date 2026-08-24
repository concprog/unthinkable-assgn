"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import { canTransition, statusLabel, type OrderStatus } from "@/lib/status";

type AgentOrder = {
  id: string;
  order_number: string;
  status: OrderStatus;
  drop_address?: { line1?: string; city?: string; pincode?: string };
  customer_name?: string;
  payment_type?: string;
  order_value?: number;
};

const ACTION_FLOW: OrderStatus[] = ["PICKED_UP", "IN_TRANSIT", "OUT_FOR_DELIVERY", "DELIVERED"];

export default function AgentOrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<AgentOrder | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showFailReason, setShowFailReason] = useState(false);
  const [failReason, setFailReason] = useState("");

  async function load() {
    try {
      setOrder(await apiFetch<AgentOrder>(`/orders/${id}`));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load order");
    }
  }

  useEffect(() => {
    // fetch-on-mount; state updates happen after await
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function updateStatus(status: OrderStatus, notes?: string) {
    setBusy(true);
    setError(null);
    try {
      await apiFetch(`/orders/${id}/status`, {
        method: "PATCH",
        body: { status, actor_type: "AGENT", notes },
      });
      await load();
      setShowFailReason(false);
      setFailReason("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Update failed");
    } finally {
      setBusy(false);
    }
  }

  if (error && !order) {
    return <Main><p className="text-red-600">{error}</p></Main>;
  }
  if (!order) {
    return <Main><p className="text-zinc-400">Loading…</p></Main>;
  }

  return (
    <Main>
      <div className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm space-y-5">
        <div>
          <Link href="/agent" className="text-sm text-blue-600 hover:underline">← My deliveries</Link>
          <h1 className="mt-2 text-lg font-mono font-semibold">
            #{order.order_number}
            {order.drop_address?.line1 && ` → ${order.drop_address.line1}`}
          </h1>
          <p className="text-sm text-zinc-500 mt-1">
            {order.customer_name ?? "Customer"}
            {" · "}
            {order.payment_type === "COD" ? `COD ₹${order.order_value?.toFixed(0) ?? ""}` : "Prepaid"}
            {" · "}
            {order.drop_address?.pincode}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-2">
          {ACTION_FLOW.map((status) => (
            <button
              key={status}
              disabled={!canTransition(order.status, status) || busy}
              onClick={() => updateStatus(status)}
              className="rounded-md border border-zinc-300 py-2.5 text-sm font-medium hover:bg-zinc-50 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              {statusLabel(status)}
            </button>
          ))}
        </div>

        {!showFailReason ? (
          <button
            disabled={!canTransition(order.status, "FAILED") || busy}
            onClick={() => setShowFailReason(true)}
            className="w-full rounded-md border border-red-300 py-2.5 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-30 disabled:cursor-not-allowed"
          >
            Mark Failed
          </button>
        ) : (
          <div className="space-y-2">
            <label htmlFor="failure_reason" className="text-xs font-medium text-zinc-600 uppercase tracking-wide">
              Failure reason (required)
            </label>
            <input
              id="failure_reason"
              value={failReason}
              onChange={(e) => setFailReason(e.target.value)}
              placeholder='e.g. "Customer unreachable"'
              className="w-full rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-black focus:outline-none"
            />
            <button
              disabled={!failReason.trim() || busy}
              onClick={() => updateStatus("FAILED", failReason.trim())}
              className="w-full rounded-md bg-red-600 py-2.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-40"
            >
              Confirm failure
            </button>
          </div>
        )}

        {error && <p className="text-sm text-red-600">{error}</p>}
      </div>
    </Main>
  );
}

function Main({ children }: { children: React.ReactNode }) {
  return <main className="mx-auto w-full max-w-md flex-1 p-6">{children}</main>;
}
