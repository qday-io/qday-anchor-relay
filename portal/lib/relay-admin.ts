/**
 * Client for the relay's internal admin HTTP API. Server-side ONLY: the API
 * is unauthenticated inside the compose network, so every portal entry point
 * that reaches it must sit behind the portal's session check, and
 * RELAY_ADMIN_URL must never be exposed to the browser.
 */

export interface WalletAddress {
  address_hex: string;
  /**
   * Optional 68-byte short form (~136 hex chars, starts "abe0…"). When
   * present, wallets can resolve it to the full address, so it is shown as
   * the primary funding address.
   */
  short_address_hex?: string;
  network: string;
  note?: string;
  /** 2 = pseudonymous, 1 = full-privacy. */
  privacy_level?: number;
  /** "mnemonic" | "keyfile" */
  wallet_source?: string;
  /**
   * true (mnemonic wallets): address_hex is deterministic — identical on
   * every request and across restarts. false or missing (keyfile wallets):
   * the address rotates per request.
   */
  stable?: boolean;
}

/** Readable label for the relay wallet's privacy level. Pure for testing. */
export function privacyLevelLabel(level: number | undefined): string {
  if (level === 2) return "Pseudonymous（假名）";
  if (level === 1) return "Full-privacy（完全隱私）";
  return "—";
}

/** Readable label for the relay wallet's key source. Pure for testing. */
export function walletSourceLabel(source: string | undefined): string {
  if (source === "mnemonic") return "Mnemonic（助記詞）";
  if (source === "keyfile") return "Key file（金鑰檔）";
  return "—";
}

export interface GeneratedAccount {
  mnemonic: string;
  address_hex: string;
  network: string;
  privacy_level: number;
}

/**
 * Resolve the relay admin base URL. Pure for unit testing:
 * - RELAY_ADMIN_URL set -> use it (trailing slashes stripped)
 * - unset in production (compose network) -> http://relay:8081
 * - unset in local dev -> http://localhost:8081
 */
export function resolveRelayAdminUrl(
  envValue: string | undefined,
  nodeEnv: string | undefined,
): string {
  const trimmed = envValue?.trim().replace(/\/+$/, "");
  if (trimmed) return trimmed;
  return nodeEnv === "production" ? "http://relay:8081" : "http://localhost:8081";
}

function relayAdminUrl(): string {
  return resolveRelayAdminUrl(process.env.RELAY_ADMIN_URL, process.env.NODE_ENV);
}

export async function fetchWalletAddress(): Promise<WalletAddress> {
  const res = await fetch(`${relayAdminUrl()}/admin/wallet-address`, {
    cache: "no-store",
    signal: AbortSignal.timeout(5_000),
  });
  if (!res.ok) {
    throw new Error(`relay admin API responded with status ${res.status}`);
  }
  const data = (await res.json()) as WalletAddress;
  if (typeof data?.address_hex !== "string" || data.address_hex.length === 0) {
    throw new Error("relay admin API returned an unexpected payload");
  }
  return data;
}

export async function generateAccount(): Promise<GeneratedAccount> {
  const res = await fetch(`${relayAdminUrl()}/admin/generate-account`, {
    method: "POST",
    cache: "no-store",
    signal: AbortSignal.timeout(15_000),
  });
  if (!res.ok) {
    throw new Error(`relay admin API responded with status ${res.status}`);
  }
  const data = (await res.json()) as GeneratedAccount;
  if (
    typeof data?.mnemonic !== "string" ||
    data.mnemonic.length === 0 ||
    typeof data?.address_hex !== "string" ||
    data.address_hex.length === 0
  ) {
    // Deliberately generic: never include the payload (mnemonic) in errors/logs.
    throw new Error("relay admin API returned an unexpected payload");
  }
  return data;
}
