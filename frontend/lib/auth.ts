export type Role = "customer" | "agent" | "admin";

const TOKEN_KEY = "auth_token";
const ROLE_COOKIE = "role";

export function homeForRole(role: string): string {
  switch (role) {
    case "customer":
      return "/dashboard";
    case "agent":
      return "/agent";
    case "admin":
      return "/admin";
    default:
      return "/";
  }
}

export function saveSession(token: string, role: Role) {
  localStorage.setItem(TOKEN_KEY, token);
  // proxy.ts reads this cookie to gate /dashboard, /agent, /admin routes
  document.cookie = `${ROLE_COOKIE}=${role}; path=/; max-age=86400; samesite=lax`;
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY);
  document.cookie = `${ROLE_COOKIE}=; path=/; max-age=0`;
}
