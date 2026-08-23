import { NextResponse, type NextRequest } from "next/server";

const ROLE_HOME: Record<string, string> = {
  customer: "/dashboard",
  agent: "/agent",
  admin: "/admin",
};

export function proxy(request: NextRequest) {
  const role = request.cookies.get("role")?.value ?? "";
  const { pathname } = request.nextUrl;

  if (pathname.startsWith("/admin") && role !== "admin") {
    return NextResponse.redirect(new URL("/", request.url));
  }
  if (pathname.startsWith("/agent") && role !== "agent" && role !== "admin") {
    return NextResponse.redirect(new URL("/", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*", "/agent/:path*", "/admin/:path*"],
};

export function homeForRole(role: string): string {
  return ROLE_HOME[role] ?? "/";
}
