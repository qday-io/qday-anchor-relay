"use client";

import { useState, type FormEvent } from "react";

export function LoginForm() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setPending(true);
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (res.ok) {
        // Full navigation so the middleware sees the fresh session cookie.
        window.location.href = "/dashboard";
        return;
      }
      const data = await res.json().catch(() => null);
      setError(
        (data && typeof data.error === "string" && data.error) ||
          "Sign-in failed. Please try again.",
      );
      setPending(false);
    } catch {
      setError("Network error. Please try again.");
      setPending(false);
    }
  }

  return (
    // method/action: pre-hydration fallback must POST to the API route —
    // never a native GET that would put credentials in the URL.
    <form onSubmit={onSubmit} method="post" action="/api/auth/login">
      <div className="login-field">
        <label htmlFor="username">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          autoComplete="username"
          autoFocus
          required
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
      </div>
      <div className="login-field">
        <label htmlFor="password">Password</label>
        <input
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </div>
      {error ? (
        <p className="login-error" role="alert">
          {error}
        </p>
      ) : null}
      <button type="submit" className="btn btn-primary login-submit" disabled={pending}>
        {pending ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}
