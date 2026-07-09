import { NextResponse } from "next/server";
import { generateAccount } from "@/lib/relay-admin";
import { isAuthenticated } from "@/lib/session";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

/**
 * Session-protected proxy to the relay's internal admin API. The relay API
 * itself is unauthenticated inside the compose network, so this handler MUST
 * verify the iron-session before proxying. The response body (mnemonic) is
 * never logged and never cached.
 */
export async function POST() {
  if (!(await isAuthenticated())) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const account = await generateAccount();
    return NextResponse.json(account, {
      headers: { "Cache-Control": "no-store" },
    });
  } catch {
    // Generic error only — no upstream payload, no mnemonic, no URL details.
    return NextResponse.json(
      { error: "無法連線 relay admin API，請確認 relay 服務狀態後再試。" },
      { status: 502, headers: { "Cache-Control": "no-store" } },
    );
  }
}
