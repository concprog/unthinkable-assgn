"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";
import { homeForRole, saveSession, type Role } from "@/lib/auth";

type AuthResponse = {
  token: string;
  email_verified?: boolean;
  user: { id: string; role: Role; full_name: string };
};

// useSearchParams needs a Suspense boundary for the static prerender.
export default function LoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [form, setForm] = useState({
    full_name: "",
    email: "",
    phone: "",
    password: "",
    role: "customer" as Role,
  });
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(
    params.get("expired") ? "Your session expired — please sign in again." : null
  );
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // prefill email when coming back to verify an account
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMode(params.get("mode") === "register" ? "register" : "login");
  }, [params]);

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

      if (res.email_verified === false) {
        // best-effort; failure is non-blocking (placeholder Resend key)
        apiFetch("/auth/send-verification", { method: "POST" }).catch(() => {});
        setNotice("Account created. We sent a verification link to your email.");
      }

      const next = params.get("next");
      router.push(next && next.startsWith("/") ? next : homeForRole(res.user.role));
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

        {notice && (
          <p className="rounded-md bg-blue-50 px-3 py-2 text-sm text-blue-700">{notice}</p>
        )}
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
