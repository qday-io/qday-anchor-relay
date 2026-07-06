/**
 * In-memory login rate limiter (single-instance deployment).
 *
 * Policy: after `maxFailures` consecutive failed attempts for a key (client
 * IP), the key is locked for `lockoutMs`. A successful login resets the
 * counter. Once a lock expires the counter starts fresh.
 *
 * Pure state machine with an injectable clock so it is unit-testable.
 */

export interface RateLimiterOptions {
  maxFailures?: number;
  lockoutMs?: number;
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
}

export const DEFAULT_MAX_FAILURES = 5;
export const DEFAULT_LOCKOUT_MS = 15 * 60 * 1000;

export class LoginRateLimiter {
  private readonly maxFailures: number;
  private readonly lockoutMs: number;
  private readonly entries = new Map<string, Entry>();

  constructor(options: RateLimiterOptions = {}) {
    this.maxFailures = options.maxFailures ?? DEFAULT_MAX_FAILURES;
    this.lockoutMs = options.lockoutMs ?? DEFAULT_LOCKOUT_MS;
  }

  /** Whether an attempt for `key` is currently allowed. */
  check(key: string, now: number = Date.now()): CheckResult {
    const entry = this.entries.get(key);
    if (!entry) {
      return { allowed: true, retryAfterMs: 0, remaining: this.maxFailures };
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
      return { allowed: true, retryAfterMs: 0, remaining: this.maxFailures };
    }
    return {
      allowed: true,
      retryAfterMs: 0,
      remaining: this.maxFailures - entry.failures,
    };
  }

  /** Record a failed attempt. Returns the state after recording. */
  recordFailure(key: string, now: number = Date.now()): CheckResult {
    const state = this.check(key, now);
    if (!state.allowed) {
      return state;
    }
    const entry = this.entries.get(key) ?? { failures: 0, lockedUntil: 0 };
    entry.failures += 1;
    if (entry.failures >= this.maxFailures) {
      entry.lockedUntil = now + this.lockoutMs;
    }
    this.entries.set(key, entry);
    return this.check(key, now);
  }

  /** Record a successful attempt: clears any failure history for `key`. */
  recordSuccess(key: string): void {
    this.entries.delete(key);
  }

  /** Number of keys currently tracked (for tests). */
  size(): number {
    return this.entries.size;
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
