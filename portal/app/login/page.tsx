import type { Metadata } from "next";
import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage() {
  return (
    <div className="login-wrap">
      <div className="login-card">
        <div className="login-brand">
          QDAY <span style={{ color: "var(--purple)" }}>Anchor</span> Portal
        </div>
        <p className="login-sub">Sign in with the admin account.</p>
        <LoginForm />
      </div>
    </div>
  );
}
