/**
 * In-memory login rate limiter (single-instance deployment).
 *
 * Keying strategy (see resolveLoginKey):
 *
 * - TRUST_PROXY_HEADERS=true — the portal sits behind a trusted reverse
 *   proxy, so x-forwarded-for / x-real-ip are reliable: rate-limit per
 *   client IP, 5 failures -> 15 minute lock.
 * - TRUST_PROXY_HEADERS unset/false (default) — forwarding headers are
 *   attacker-controlled and MUST be ignored. All attempts share one global
 *   bucket with a higher threshold (20 failures / 15 minutes) so a single
 *   attacker cannot trivially lock the admin out while brute force is still
 *   throttled.
 *
 * A successful login resets the counter. Once a lock expires the counter
 * starts fresh. The map is bounded: when it grows past `maxEntries`, expired
 * locks and idle (non-locked, inactive for longer than the lockout window)
 * entries are swept.
 *
 * Pure state machine with an injectable clock so it is unit-testable.
 */

export interface RateLimiterOptions {
  maxFailures?: number;
  lockoutMs?: number;
  maxEntries?: number;
}

export interface CheckResult {
  allowed: boolean;
  /** Milliseconds until the lock expires; 0 when allowed. */
  retryAfterMs: number;
  /** Failed attempts remaining before lockout. */
  remaining: number;
}

interface Entry {
  failures: number;
  lockedUntil: number; // epoch ms, 0 when not locked
  lastSeen: number; // epoch ms of the last recorded failure
}

export const DEFAULT_MAX_FAILURES = 5;
export const DEFAULT_LOCKOUT_MS = 15 * 60 * 1000;
export const DEFAULT_MAX_ENTRIES = 10_000;

/** Shared bucket used when proxy headers are untrusted. */
export const GLOBAL_LIMIT_KEY = "global";
export const GLOBAL_MAX_FAILURES = 20;

export interface LoginKeyContext {
  /** Whether x-forwarded-for / x-real-ip come from a trusted proxy. */
  trustProxyHeaders: boolean;
  forwardedFor: string | null;
  realIp: string | null;
}

/**
 * Decide which limiter bucket an attempt belongs to and the failure
 * threshold for that bucket. Pure so it is unit-testable.
 */
export function resolveLoginKey(ctx: LoginKeyContext): {
  key: string;
  maxFailures: number;
} {
  if (!ctx.trustProxyHeaders) {
    return { key: GLOBAL_LIMIT_KEY, maxFailures: GLOBAL_MAX_FAILURES };
  }
  const forwarded = ctx.forwardedFor?.split(",")[0]?.trim();
  const ip = forwarded || ctx.realIp?.trim() || "unknown";
  return { key: `ip:${ip}`, maxFailures: DEFAULT_MAX_FAILURES };
}

export class LoginRateLimiter {
  private readonly maxFailures: number;
  private readonly lockoutMs: number;
  private readonly maxEntries: number;
  private readonly entries = new Map<string, Entry>();

  constructor(options: RateLimiterOptions = {}) {
    this.maxFailures = options.maxFailures ?? DEFAULT_MAX_FAILURES;
    this.lockoutMs = options.lockoutMs ?? DEFAULT_LOCKOUT_MS;
    this.maxEntries = options.maxEntries ?? DEFAULT_MAX_ENTRIES;
  }

  /**
   * Whether an attempt for `key` is currently allowed. `maxFailures`
   * overrides the instance default for this bucket (used for the global
   * bucket's higher threshold).
   */
  check(
    key: string,
    now: number = Date.now(),
    maxFailures: number = this.maxFailures,
  ): CheckResult {
    this.sweepIfNeeded(now);
    const entry = this.entries.get(key);
    if (!entry) {
      return { allowed: true, retryAfterMs: 0, remaining: maxFailures };
    }
    if (entry.lockedUntil > now) {
      return {
        allowed: false,
        retryAfterMs: entry.lockedUntil - now,
        remaining: 0,
      };
    }
    if (entry.lockedUntil !== 0) {
      // Lock expired: start fresh.
      this.entries.delete(key);
      return { allowed: true, retryAfterMs: 0, remaining: maxFailures };
    }
    return {
      allowed: true,
      retryAfterMs: 0,
      remaining: Math.max(0, maxFailures - entry.failures),
    };
  }

  /** Record a failed attempt. Returns the state after recording. */
  recordFailure(
    key: string,
    now: number = Date.now(),
    maxFailures: number = this.maxFailures,
  ): CheckResult {
    const state = this.check(key, now, maxFailures);
    if (!state.allowed) {
      return state;
    }
    const entry = this.entries.get(key) ?? {
      failures: 0,
      lockedUntil: 0,
      lastSeen: now,
    };
    entry.failures += 1;
    entry.lastSeen = now;
    if (entry.failures >= maxFailures) {
      entry.lockedUntil = now + this.lockoutMs;
    }
    this.entries.set(key, entry);
    return this.check(key, now, maxFailures);
  }

  /** Record a successful attempt: clears any failure history for `key`. */
  recordSuccess(key: string): void {
    this.entries.delete(key);
  }

  /** Number of keys currently tracked (for tests). */
  size(): number {
    return this.entries.size;
  }

  /**
   * Bound memory: once the map outgrows `maxEntries`, drop entries whose
   * lock has expired and non-locked entries idle for longer than the
   * lockout window. Active locks are never dropped.
   */
  private sweepIfNeeded(now: number): void {
    if (this.entries.size <= this.maxEntries) return;
    for (const [key, entry] of this.entries) {
      const lockExpired = entry.lockedUntil !== 0 && entry.lockedUntil <= now;
      const staleIdle =
        entry.lockedUntil === 0 && now - entry.lastSeen >= this.lockoutMs;
      if (lockExpired || staleIdle) {
        this.entries.delete(key);
      }
    }
  }
}

// Module-level singleton shared by the login route handler. Stored on
// globalThis so dev-server hot reloads do not reset the limiter.
const globalStore = globalThis as typeof globalThis & {
  __portalLoginRateLimiter?: LoginRateLimiter;
};

export function getLoginRateLimiter(): LoginRateLimiter {
  if (!globalStore.__portalLoginRateLimiter) {
    globalStore.__portalLoginRateLimiter = new LoginRateLimiter();
  }
  return globalStore.__portalLoginRateLimiter;
}
