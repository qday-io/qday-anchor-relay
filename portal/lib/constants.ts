// Kept in its own module (no next/headers import) so middleware — which runs
// in the edge runtime — can import the cookie name without pulling in
// server-only modules.
export const SESSION_COOKIE_NAME = "qday_portal_session";
