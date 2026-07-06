import { getIronSession, type IronSession, type SessionOptions } from "iron-session";
import { cookies } from "next/headers";
import { SESSION_COOKIE_NAME } from "./constants";

export interface SessionData {
  username?: string;
  loggedInAt?: number;
}

export function sessionOptions(): SessionOptions {
  const secret = process.env.SESSION_SECRET;
  if (!secret || secret.length < 32) {
    throw new Error("SESSION_SECRET must be set and at least 32 characters long");
  }
  return {
    cookieName: SESSION_COOKIE_NAME,
    password: secret,
    cookieOptions: {
      httpOnly: true,
      sameSite: "lax",
      secure: process.env.NODE_ENV === "production",
      path: "/",
    },
  };
}

/** Read/write the iron-session for the current request (server-side only). */
export async function getSession(): Promise<IronSession<SessionData>> {
  return getIronSession<SessionData>(await cookies(), sessionOptions());
}

/** True when the current request carries a valid, unsealed admin session. */
export async function isAuthenticated(): Promise<boolean> {
  const session = await getSession();
  return typeof session.username === "string" && session.username.length > 0;
}
