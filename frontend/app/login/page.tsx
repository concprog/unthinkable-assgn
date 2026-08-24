"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import { homeForRole, saveSession, type Role } from "@/lib/auth";

type AuthResponse = {
  token: string;
  user: { id: string; role: Role; full_name: string };
};

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [form, setForm] = useState({
    full_name: "",
    email: "",
    phone: "",
    password: "",
    role: "customer" as Role,
  });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function set<K extends keyof typeof form>(key: K, value: string) {
    setForm((f) => ({ ...f, [key]: value }));
    setError(null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const path = mode === "login" ? "/auth/login" : "/auth/register";
      const body =
        mode === "login"
          ? { email: form.email, password: form.password }
          : form;
      const res = await apiFetch<AuthResponse>(path, { method: "POST", body });
      saveSession(res.token, res.user.role);
      router.push(homeForRole(res.user.role));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Request failed");
      setBusy(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center p-6">
      <form
        onSubmit={submit}
        className="w-full max-w-sm rounded-lg border border-zinc-200 bg-white p-6 shadow-sm space-y-4"
      >
        <h1 className="text-xl font-semibold">
          {mode === "login" ? "Sign in" : "Create account"}
        </h1>

        {mode === "register" && (
          <>
            <Field label="Full name" id="full_name" value={form.full_name} onChange={(v) => set("full_name", v)} placeholder="R. Kumar" />
            <Field label="Phone" id="phone" value={form.phone} onChange={(v) => set("phone", v)} placeholder="+91 98765 43210" />
            <label className="block text-sm">
              <span className="text-xs font-medium text-zinc-600 uppercase tracking-wide">Role</span>
              <select
                value={form.role}
                onChange={(e) => set("role", e.target.value)}
                className="mt-1 w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm focus:border-black focus:outline-none"
              >
                <option value="customer">Customer</option>
                <option value="agent">Delivery agent</option>
                <option value="admin">Admin</option>
              </select>
            </label>
          </>
        )}
        <Field label="Email" id="email" type="email" value={form.email} onChange={(v) => set("email", v)} placeholder="you@example.com" />
        <Field label="Password" id="password" type="password" value={form.password} onChange={(v) => set("password", v)} placeholder="••••••••" />

        {error && <p className="text-sm text-red-600">{error}</p>}

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-md bg-black py-2.5 text-white text-sm font-medium hover:bg-zinc-800 disabled:opacity-50"
        >
          {busy ? "…" : mode === "login" ? "Sign in" : "Sign up"}
        </button>

        <button
          type="button"
          onClick={() => { setMode(mode === "login" ? "register" : "login"); setError(null); }}
          className="w-full text-center text-sm text-blue-600 hover:underline"
        >
          {mode === "login" ? "No account? Create one" : "Have an account? Sign in"}
        </button>
      </form>
    </main>
  );
}

function Field({
  label, id, value, onChange, type = "text", placeholder,
}: {
  label: string;
  id: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
}) {
  return (
    <label className="block text-sm">
      <span className="text-xs font-medium text-zinc-600 uppercase tracking-wide">{label}</span>
      <input
        id={id}
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        required
        className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-900 focus:border-black focus:outline-none"
      />
    </label>
  );
}
