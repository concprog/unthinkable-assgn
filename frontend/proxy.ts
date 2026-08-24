import { NextResponse, type NextRequest } from "next/server";

const ROLE_HOME: Record<string, string> = {
  customer: "/dashboard",
  agent: "/agent",
  admin: "/admin",
};

// Edge gate for /dashboard, /agent, /admin. No session at all →
// /login?next=<path> so the user lands back where they started after
// signing in. Wrong role → bounced to their own home page.
export function proxy(request: NextRequest) {
  const role = request.cookies.get("role")?.value ?? "";
  const { pathname, search } = request.nextUrl;

  const area =
    pathname.startsWith("/admin") || pathname.startsWith("/agent")
      ? "staff"
      : "customer";

  if (area === "customer" && role !== "customer" && role !== "admin") {
    return deny(request, pathname + search);
  }
  if (area === "staff" && pathname.startsWith("/admin") && role !== "admin") {
    // logged in as the wrong role → own home; anonymous → login
    return ROLE_HOME[role]
      ? NextResponse.redirect(new URL(ROLE_HOME[role], request.url))
      : deny(request, pathname + search);
  }

  return NextResponse.next();
}

function deny(request: NextRequest, next: string): NextResponse {
  const url = new URL("/login", request.url);
  if (next && next !== "/") url.searchParams.set("next", next);
  return NextResponse.redirect(url);
}

export const config = {
  matcher: ["/dashboard/:path*", "/agent/:path*", "/admin/:path*"],
};

export function homeForRole(role: string): string {
  return ROLE_HOME[role] ?? "/";
}
