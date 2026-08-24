"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { clearSession } from "@/lib/auth";

export function LogoutButton() {
  const router = useRouter();
  const [signedIn, setSignedIn] = useState(false);

  useEffect(() => {
    // localStorage only exists client-side; read it post-mount
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSignedIn(!!localStorage.getItem("auth_token"));
  }, []);

  if (!signedIn) return null;

  return (
    <button
      onClick={() => {
        clearSession();
        router.push("/");
        router.refresh();
      }}
      className="rounded-md border border-zinc-300 px-3 py-1 text-xs text-zinc-600 hover:bg-zinc-50"
    >
      Sign out
    </button>
  );
}
