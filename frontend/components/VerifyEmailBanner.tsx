"use client";

import { useEffect, useState } from "react";
import { apiFetch, ApiError } from "@/lib/api";

// Reads the signed-in profile; if email_verified is false shows a
// banner with a Resend action. Hidden entirely when verified or
// signed out.
export function VerifyEmailBanner() {
  const [unverified, setUnverified] = useState(false);
  const [state, setState] = useState<"idle" | "sending" | "sent" | "error">("idle");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ email_verified?: boolean }>("/auth/me")
      .then((me) => {
        if (!cancelled) setUnverified(me.email_verified === false);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  if (!unverified) return null;

  async function resend() {
    setState("sending");
    setError(null);
    try {
      await apiFetch("/auth/send-verification", { method: "POST" });
      setState("sent");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not send email");
      setState("error");
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
      <span>
        Please verify your email address to receive delivery notifications.
      </span>
      {state === "sent" ? (
        <span className="font-medium">Verification email sent — check your inbox.</span>
      ) : (
        <button
          onClick={resend}
          disabled={state === "sending"}
          className="rounded-md border border-amber-300 bg-white px-3 py-1 text-xs font-medium text-amber-800 hover:bg-amber-100 disabled:opacity-50"
        >
          {state === "sending" ? "Sending…" : "Resend verification email"}
        </button>
      )}
      {error && <span className="text-red-600">{error}</span>}
    </div>
  );
}
