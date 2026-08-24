"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { apiFetch, ApiError } from "@/lib/api";

export default function VerifyPage() {
  return (
    <Suspense>
      <VerifyInner />
    </Suspense>
  );
}

function VerifyInner() {
  const params = useSearchParams();
  const [state, setState] = useState<"verifying" | "ok" | "error">("verifying");
  const [message, setMessage] = useState<string | null>(null);
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return;
    ran.current = true;

    const token = params.get("token");
    if (!token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setState("error");
      setMessage("Missing verification token.");
      return;
    }

    apiFetch<{ ok: boolean }>(`/auth/verify?token=${encodeURIComponent(token)}`)
      .then(() => {
        setState("ok");
      })
      .catch((err) => {
        setMessage(err instanceof ApiError ? err.message : "Verification failed");
        setState("error");
      });
  }, [params]);

  return (
    <main className="flex flex-1 items-center justify-center p-6">
      <div className="w-full max-w-sm rounded-lg border border-zinc-200 bg-white p-6 text-center shadow-sm">
        <h1 className="mb-2 text-xl font-semibold">Email verification</h1>

        {state === "verifying" && <p className="text-sm text-zinc-500">Checking your link…</p>}

        {state === "ok" && (
          <>
            <p className="rounded-md bg-green-50 px-3 py-2 text-sm text-green-700">
              Your email is verified. Everything is set.
            </p>
            <Link href="/dashboard" className="mt-4 inline-block text-sm text-blue-600 hover:underline">
              Go to your dashboard →
            </Link>
          </>
        )}

        {state === "error" && (
          <>
            {message && <p className="mb-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{message}</p>}
            <p className="text-sm text-zinc-500">
              The link may have expired or already been used.
            </p>
            <Link href="/login" className="mt-4 inline-block text-sm text-blue-600 hover:underline">
              Sign in and resend the email
            </Link>
          </>
        )}
      </div>
    </main>
  );
}
