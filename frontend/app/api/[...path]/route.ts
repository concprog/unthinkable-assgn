// Runtime proxy: forwards same-origin /api/* calls to the Go backend.
// BACKEND_API_URL is read per-request from the server environment, so
// changing the backend domain needs only a restart/redeploy — never an
// image rebuild (unlike rewrites, which standalone freezes at build).
const HEADER_WHITELIST = ["authorization", "content-type"];

function backendBase(): string {
  return process.env.BACKEND_API_URL ?? "http://localhost:8080";
}

async function forward(req: Request, path: string[]): Promise<Response> {
  const search = new URL(req.url).search;
  const target = `${backendBase()}/api/${path.join("/")}${search}`;

  const headers = new Headers();
  for (const name of HEADER_WHITELIST) {
    const value = req.headers.get(name);
    if (value) headers.set(name, value);
  }

  const hasBody = req.method !== "GET" && req.method !== "HEAD";
  const body = hasBody ? await req.arrayBuffer() : undefined;

  try {
    const upstream = await fetch(target, { method: req.method, headers, body });
    const resHeaders = new Headers();
    const contentType = upstream.headers.get("content-type");
    if (contentType) resHeaders.set("content-type", contentType);
    return new Response(upstream.body, {
      status: upstream.status,
      headers: resHeaders,
    });
  } catch {
    return Response.json({ error: "backend unavailable" }, { status: 502 });
  }
}

type Ctx = { params: Promise<{ path: string[] }> };

export const dynamic = "force-dynamic";

export async function GET(req: Request, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function POST(req: Request, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function PATCH(req: Request, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function PUT(req: Request, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
export async function DELETE(req: Request, ctx: Ctx) {
  return forward(req, (await ctx.params).path);
}
