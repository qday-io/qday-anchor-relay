import type { Metadata } from "next";
import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Sign in" };

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const sp = await searchParams;
  // Set by the no-JS form flow (303 redirect to /login?error=1 on failure).
  // Deliberately generic — no detail about which credential was wrong.
  const initialError =
    sp.error === "1" ? "Sign-in failed. Please try again." : undefined;

  return (
    <div className="login-wrap">
      <div className="login-card">
        <div className="login-brand">
          QDAY <span style={{ color: "var(--purple)" }}>Anchor</span> Portal
        </div>
        <p className="login-sub">Sign in with the admin account.</p>
        <LoginForm initialError={initialError} />
      </div>
    </div>
  );
}
