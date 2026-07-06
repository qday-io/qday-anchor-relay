/**
 * Anchor memo protocol (must stay byte-for-byte compatible with the Go relay):
 *
 *   QDAY2ANCHOR|v1|<batchNumber decimal>|0x<stateRoot 64 hex lowercase>
 */

export const MEMO_PREFIX = "QDAY2ANCHOR";
export const MEMO_VERSION = "v1";

const UINT64_MAX = (1n << 64n) - 1n;
const BATCH_RE = /^[0-9]+$/;
const ROOT_RE = /^[0-9a-f]{64}$/;

export class MemoError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "MemoError";
  }
}

export interface DecodedMemo {
  batchNumber: bigint;
  /** 0x-prefixed, 64 lowercase hex characters. */
  stateRoot: string;
}

/**
 * Encode a memo string. `stateRootHex` may be given with or without a `0x`
 * prefix, but must be exactly 64 lowercase hex characters.
 */
export function encodeMemo(batchNumber: bigint, stateRootHex: string): string {
  if (batchNumber < 0n) {
    throw new MemoError("batch number must not be negative");
  }
  if (batchNumber > UINT64_MAX) {
    throw new MemoError("batch number exceeds uint64 range");
  }
  const root = stateRootHex.startsWith("0x")
    ? stateRootHex.slice(2)
    : stateRootHex;
  if (!ROOT_RE.test(root)) {
    throw new MemoError(
      "state root must be exactly 64 lowercase hex characters",
    );
  }
  return `${MEMO_PREFIX}|${MEMO_VERSION}|${batchNumber.toString(10)}|0x${root}`;
}

/** Decode and strictly validate a memo string. Throws MemoError on any deviation. */
export function decodeMemo(memo: string): DecodedMemo {
  const parts = memo.split("|");
  if (parts.length !== 4) {
    throw new MemoError("memo must have exactly 4 |-separated segments");
  }
  const [prefix, version, batchStr, rootStr] = parts;
  if (prefix !== MEMO_PREFIX) {
    throw new MemoError(`invalid memo prefix: ${JSON.stringify(prefix)}`);
  }
  if (version !== MEMO_VERSION) {
    throw new MemoError(`unsupported memo version: ${JSON.stringify(version)}`);
  }
  if (!BATCH_RE.test(batchStr)) {
    throw new MemoError("batch number must be a non-negative decimal integer");
  }
  const batchNumber = BigInt(batchStr);
  if (batchNumber > UINT64_MAX) {
    throw new MemoError("batch number exceeds uint64 range");
  }
  if (!rootStr.startsWith("0x")) {
    throw new MemoError("state root must be 0x-prefixed");
  }
  const root = rootStr.slice(2);
  if (!ROOT_RE.test(root)) {
    throw new MemoError(
      "state root must be exactly 64 lowercase hex characters",
    );
  }
  return { batchNumber, stateRoot: `0x${root}` };
}
