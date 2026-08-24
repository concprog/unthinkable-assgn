// Same-origin relative base: the Next.js server proxies /api/* to the
// Go backend via rewrites in next.config.ts (BACKEND_API_URL env, read
// at server start). The browser never needs to know the backend URL,
// so no rebuild is required when it changes — and CORS is moot.
const API_BASE = "/api";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

type TokenGetter = () => Promise<string | null>;

// Default: the first-party session token stashed by /login.
let getToken: TokenGetter = async () => {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("auth_token");
};

export function setTokenGetter(fn: TokenGetter) {
  getToken = fn;
}

export async function apiFetch<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  const token = await getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    method: options.method ?? "GET",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {}

    // Expired/invalid session on a protected page: drop the stale
    // token and send the user to login. Login/register failures (no
    // token stashed) surface the error inline instead.
    if (res.status === 401 && typeof window !== "undefined") {
      const hadToken = localStorage.getItem("auth_token");
      const onAuthPage = window.location.pathname === "/login";
      if (hadToken && !onAuthPage) {
        const { clearSession } = await import("./auth");
        clearSession();
        // full navigation is intentional: drops all client state
        // (no router available outside a component)
        const next = encodeURIComponent(
          window.location.pathname + window.location.search
        );
        // eslint-disable-next-line @next/next/no-location-assign-relative-destination
        window.location.assign(`/login?next=${next}&expired=1`);
      }
    }

    throw new ApiError(res.status, message);
  }

  return res.json() as Promise<T>;
}
