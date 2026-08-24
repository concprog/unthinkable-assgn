"use client";

import Link from "next/link";

const ROLES = [
  {
    href: "/dashboard",
    title: "Customer",
    desc: "Place a delivery, see the charge breakdown, track it live.",
  },
  {
    href: "/agent",
    title: "Delivery Agent",
    desc: "Your task list, availability toggle, one-tap status updates.",
  },
  {
    href: "/admin",
    title: "Admin",
    desc: "Orders dashboard, zones & rate cards, assignment overrides.",
  },
];

export default function Home() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-8 p-8">
      <h1 className="text-3xl font-semibold tracking-tight">Last-Mile Delivery Tracker</h1>
      <div className="grid w-full max-w-3xl grid-cols-1 gap-4 sm:grid-cols-3">
        {ROLES.map((r) => (
          <Link
            key={r.href}
            href={r.href}
            className="rounded-xl border border-zinc-200 bg-white p-6 shadow-sm transition hover:shadow-md"
          >
            <div className="font-semibold">{r.title}</div>
            <p className="mt-2 text-sm text-zinc-500">{r.desc}</p>
          </Link>
        ))}
      </div>
    </main>
  );
}
