"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "@/lib/api";

type OrderSummary = {
  id: string;
  order_number: string;
  status: string;
  total_charge: number;
  created_at: string;
};

export default function MyOrdersPage() {
  const [orders, setOrders] = useState<OrderSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiFetch<OrderSummary[]>("/orders")
      .then(setOrders)
      .catch((err) => setError(err instanceof ApiError ? err.message : "Failed to load orders"));
  }, []);

  return (
    <main className="mx-auto w-full max-w-md flex-1 p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-semibold">My orders</h1>
        <Link href="/dashboard" className="text-sm text-blue-600 hover:underline">+ New delivery</Link>
      </div>
      <div className="rounded-lg border border-zinc-200 bg-white shadow-sm divide-y divide-zinc-100 text-sm">
        {error && <p className="p-4 text-red-600">{error}</p>}
        {!error && orders.length === 0 && <p className="p-4 text-zinc-400">No orders yet.</p>}
        {orders.map((o) => (
          <Link key={o.id} href={`/dashboard/orders/${o.id}`} className="flex items-center justify-between px-4 py-3 hover:bg-zinc-50">
            <span className="font-mono font-medium">{o.order_number}</span>
            <span className="text-zinc-500">{new Date(o.created_at).toLocaleDateString()}</span>
            <span>₹ {Number(o.total_charge).toFixed(2)}</span>
          </Link>
        ))}
      </div>
    </main>
  );
}
