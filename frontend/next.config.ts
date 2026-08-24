import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // /api/* is proxied by app/api/[...path]/route.ts instead of rewrites:
  // standalone output freezes rewrites() at build time, while the route
  // handler reads BACKEND_API_URL from the environment at request time.
};

export default nextConfig;
