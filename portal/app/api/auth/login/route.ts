import { NextResponse, type NextRequest } from "next/server";
import { verifyCredentials } from "@/lib/auth";
import { getLoginRateLimiter, resolveLoginKey } from "@/lib/ratelimit";
import { getSession } from "@/lib/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// TRUST_PROXY_HEADERS: set to "true" ONLY when the portal runs behind a
// trusted reverse proxy that overwrites x-forwarded-for. Then failed logins
// are limited per client IP (5 / 15 min). When unset (default, container
// exposed directly), forwarding headers are attacker-controlled and are
// ignored: all attempts share a single global bucket (20 / 15 min) so a
// spoofed header can neither bypass the limiter nor lock the admin out.
function trustProxyHeaders(): boolean {
  const value = (process.env.TRUST_PROXY_HEADERS ?? "").trim().toLowerCase();
  return value === "true" || value === "1";
}

export async function POST(request: NextRequest) {
  const limiter = getLoginRateLimiter();
  const { key, maxFailures } = resolveLoginKey({
    trustProxyHeaders: trustProxyHeaders(),
    forwardedFor: request.headers.get("x-forwarded-for"),
    realIp: request.headers.get("x-real-ip"),
  });

  const gate = limiter.check(key, Date.now(), maxFailures);
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
    limiter.recordFailure(key, Date.now(), maxFailures);
    return NextResponse.json(
      { error: "Invalid username or password." },
      { status: 401 },
    );
  }

  limiter.recordSuccess(key);
  const session = await getSession();
  session.username = username;
  session.loggedInAt = Date.now();
  await session.save();

  return NextResponse.json({ ok: true });
}
