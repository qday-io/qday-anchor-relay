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

/**
 * Accepts application/json (the hydrated client form) and
 * application/x-www-form-urlencoded (pre-hydration/no-JS native submit).
 * The form flow responds with 303 redirects because the browser needs a
 * navigation; the JSON flow keeps its JSON status responses.
 */
export async function POST(request: NextRequest) {
  const contentType = (request.headers.get("content-type") ?? "").toLowerCase();
  const isJson = contentType.includes("application/json");
  const isForm = contentType.includes("application/x-www-form-urlencoded");
  if (!isJson && !isForm) {
    // Unsupported media type: reject without burning a rate-limit failure.
    return NextResponse.json(
      { error: "Unsupported content type." },
      { status: 415 },
    );
  }

  const limiter = getLoginRateLimiter();
  const { key, maxFailures } = resolveLoginKey({
    trustProxyHeaders: trustProxyHeaders(),
    forwardedFor: request.headers.get("x-forwarded-for"),
    realIp: request.headers.get("x-real-ip"),
  });

  const formFailureRedirect = () =>
    NextResponse.redirect(new URL("/login?error=1", request.url), 303);

  const gate = limiter.check(key, Date.now(), maxFailures);
  if (!gate.allowed) {
    if (isForm) return formFailureRedirect();
    const retryAfterSeconds = Math.ceil(gate.retryAfterMs / 1000);
    return NextResponse.json(
      { error: "Too many failed attempts. Try again later." },
      { status: 429, headers: { "Retry-After": String(retryAfterSeconds) } },
    );
  }

  let username = "";
  let password = "";
  if (isJson) {
    const body = await request.json().catch(() => null);
    username = body && typeof body.username === "string" ? body.username : "";
    password = body && typeof body.password === "string" ? body.password : "";
  } else {
    const form = await request.formData().catch(() => null);
    const u = form?.get("username");
    const p = form?.get("password");
    username = typeof u === "string" ? u : "";
    password = typeof p === "string" ? p : "";
  }

  const valid = await verifyCredentials(username, password);
  if (!valid) {
    limiter.recordFailure(key, Date.now(), maxFailures);
    if (isForm) return formFailureRedirect();
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

  if (isForm) {
    return NextResponse.redirect(new URL("/dashboard", request.url), 303);
  }
  return NextResponse.json({ ok: true });
}
