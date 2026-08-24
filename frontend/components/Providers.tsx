"use client";

import type { ReactNode } from "react";

// First-party auth: the token lives in localStorage and is attached by
// lib/api.ts. No external identity provider involved.
export function Providers({ children }: { children: ReactNode }) {
  return <>{children}</>;
}
