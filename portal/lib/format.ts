const NEUTRINO_PER_ABEL = 10_000_000n; // 1 ABEL = 1e7 Neutrino

/** Render a BYTEA state root as 0x + lowercase hex. */
export function bytesToHex(bytes: Uint8Array | Buffer | null): string | null {
  if (!bytes) return null;
  return "0x" + Buffer.from(bytes).toString("hex");
}

/** Truncate a long hash in the middle: 0x3fa8b1c2d3…a5b6c21e */
export function truncateMiddle(value: string, head = 10, tail = 8): string {
  if (value.length <= head + tail + 1) return value;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}

/** Convert a Neutrino amount (bigint or numeric string) to an ABEL string. */
export function neutrinoToAbel(neutrino: bigint | string): string {
  const n = typeof neutrino === "bigint" ? neutrino : BigInt(neutrino);
  const negative = n < 0n;
  const abs = negative ? -n : n;
  const whole = abs / NEUTRINO_PER_ABEL;
  const frac = abs % NEUTRINO_PER_ABEL;
  let out = groupDigits(whole.toString());
  if (frac > 0n) {
    out += "." + frac.toString().padStart(7, "0").replace(/0+$/, "");
  }
  return (negative ? "-" : "") + out;
}

/** Group a decimal integer string with thousands separators. */
export function groupDigits(value: string): string {
  const negative = value.startsWith("-");
  const digits = negative ? value.slice(1) : value;
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return (negative ? "-" : "") + grouped;
}

/** Stable UTC timestamp rendering: 2026-07-06 10:22:33 UTC */
export function formatTimestamp(value: Date | string | null): string {
  if (!value) return "—";
  const date = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return "—";
  return date.toISOString().replace("T", " ").slice(0, 19) + " UTC";
}
