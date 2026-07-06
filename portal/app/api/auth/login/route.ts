import { NextResponse, type NextRequest } from "next/server";
import { verifyCredentials } from "@/lib/auth";
import { getLoginRateLimiter } from "@/lib/ratelimit";
import { getSession } from "@/lib/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

function clientIp(request: NextRequest): string {
  const forwarded = request.headers.get("x-forwarded-for");
  if (forwarded) {
    const first = forwarded.split(",")[0]?.trim();
    if (first) return first;
  }
  return request.headers.get("x-real-ip") ?? "unknown";
}

export async function POST(request: NextRequest) {
  const ip = clientIp(request);
  const limiter = getLoginRateLimiter();

  const gate = limiter.check(ip);
  if (!gate.allowed) {
    const retryAfterSeconds = Math.ceil(gate.retryAfterMs / 1000);
    return NextResponse.json(
      { error: "Too many failed attempts. Try again later." },
      { status: 429, headers: { "Retry-After": String(retryAfterSeconds) } },
    );
  }

  const body = await request.json().catch(() => null);
  const username =
    body && typeof body.username === "string" ? body.username : "";
  const password =
    body && typeof body.password === "string" ? body.password : "";

  const valid = await verifyCredentials(username, password);
  if (!valid) {
    limiter.recordFailure(ip);
    return NextResponse.json(
      { error: "Invalid username or password." },
      { status: 401 },
    );
  }

  limiter.recordSuccess(ip);
  const session = await getSession();
  session.username = username;
  session.loggedInAt = Date.now();
  await session.save();

  return NextResponse.json({ ok: true });
}
