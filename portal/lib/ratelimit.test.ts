import { describe, expect, it } from "vitest";
import {
  DEFAULT_LOCKOUT_MS,
  DEFAULT_MAX_FAILURES,
  LoginRateLimiter,
} from "./ratelimit";

const T0 = 1_000_000;
const IP = "203.0.113.7";

describe("LoginRateLimiter", () => {
  it("allows attempts for an unknown key", () => {
    const rl = new LoginRateLimiter();
    const res = rl.check(IP, T0);
    expect(res.allowed).toBe(true);
    expect(res.remaining).toBe(DEFAULT_MAX_FAILURES);
    expect(res.retryAfterMs).toBe(0);
  });

  it("still allows after 4 failures, locks on the 5th", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 4; i++) {
      const res = rl.recordFailure(IP, T0 + i);
      expect(res.allowed).toBe(true);
      expect(res.remaining).toBe(DEFAULT_MAX_FAILURES - i - 1);
    }
    const fifth = rl.recordFailure(IP, T0 + 4);
    expect(fifth.allowed).toBe(false);
    expect(fifth.retryAfterMs).toBe(DEFAULT_LOCKOUT_MS);
    expect(rl.check(IP, T0 + 5).allowed).toBe(false);
  });

  it("keeps the key locked for 15 minutes", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 5; i++) rl.recordFailure(IP, T0);

    const justBefore = rl.check(IP, T0 + DEFAULT_LOCKOUT_MS - 1);
    expect(justBefore.allowed).toBe(false);
    expect(justBefore.retryAfterMs).toBe(1);
  });

  it("unlocks and resets the counter after the lockout expires", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 5; i++) rl.recordFailure(IP, T0);

    const after = rl.check(IP, T0 + DEFAULT_LOCKOUT_MS);
    expect(after.allowed).toBe(true);
    expect(after.remaining).toBe(DEFAULT_MAX_FAILURES);

    // Failures start counting from zero again.
    const res = rl.recordFailure(IP, T0 + DEFAULT_LOCKOUT_MS + 1);
    expect(res.allowed).toBe(true);
    expect(res.remaining).toBe(DEFAULT_MAX_FAILURES - 1);
  });

  it("does not extend the lock for failures recorded while locked", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 5; i++) rl.recordFailure(IP, T0);
    rl.recordFailure(IP, T0 + 60_000); // attempt while locked
    expect(rl.check(IP, T0 + DEFAULT_LOCKOUT_MS).allowed).toBe(true);
  });

  it("resets the counter on success", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 4; i++) rl.recordFailure(IP, T0);
    rl.recordSuccess(IP);
    expect(rl.check(IP, T0).remaining).toBe(DEFAULT_MAX_FAILURES);
    expect(rl.size()).toBe(0);
  });

  it("tracks keys independently", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < 5; i++) rl.recordFailure(IP, T0);
    expect(rl.check(IP, T0).allowed).toBe(false);
    expect(rl.check("198.51.100.1", T0).allowed).toBe(true);
  });

  it("honors custom limits", () => {
    const rl = new LoginRateLimiter({ maxFailures: 2, lockoutMs: 100 });
    rl.recordFailure(IP, T0);
    expect(rl.recordFailure(IP, T0).allowed).toBe(false);
    expect(rl.check(IP, T0 + 99).allowed).toBe(false);
    expect(rl.check(IP, T0 + 100).allowed).toBe(true);
  });
});
