import { redirect } from "next/navigation";
import { AutoRefresh } from "@/components/AutoRefresh";
import { NavLinks } from "@/components/NavLinks";
import { isAuthenticated } from "@/lib/session";

export const dynamic = "force-dynamic";

export default async function ProtectedLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  // Real session validation (cryptographic unseal). The middleware only
  // checks that a session cookie exists.
  if (!(await isAuthenticated())) {
    redirect("/login");
  }

  return (
    <>
      <AutoRefresh />
      <header className="topnav">
        <div className="topnav-inner">
          <span className="brand">
            QDAY <span className="brand-accent">Anchor</span> Portal
          </span>
          <NavLinks />
          <form action="/api/auth/logout" method="post">
            <button type="submit" className="btn btn-quiet">
              Log out
            </button>
          </form>
        </div>
      </header>
      <main className="page">{children}</main>
    </>
  );
}
