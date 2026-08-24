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
    throw new ApiError(res.status, message);
  }

  return res.json() as Promise<T>;
}
