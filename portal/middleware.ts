import { NextResponse, type NextRequest } from "next/server";
import { SESSION_COOKIE_NAME } from "./lib/constants";

/**
 * Cheap gate: requests without a session cookie are redirected to /login.
 * Full cryptographic validation of the session (unsealing) happens in
 * app/(protected)/layout.tsx — a forged cookie passes this check but is
 * rejected there.
 */
export function middleware(request: NextRequest) {
  const hasSessionCookie = Boolean(
    request.cookies.get(SESSION_COOKIE_NAME)?.value,
  );
  if (!hasSessionCookie) {
    const loginUrl = new URL("/login", request.url);
    return NextResponse.redirect(loginUrl, 302);
  }
  return NextResponse.next();
}

export const config = {
  // Protect everything except /login, /api/auth/* and Next.js static assets.
  matcher: [
    "/((?!login|api/auth|_next/static|_next/image|favicon.ico).*)",
  ],
};
