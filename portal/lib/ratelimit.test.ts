import { describe, expect, it } from "vitest";
import {
  DEFAULT_LOCKOUT_MS,
  DEFAULT_MAX_FAILURES,
  GLOBAL_LIMIT_KEY,
  GLOBAL_MAX_FAILURES,
  LoginRateLimiter,
  resolveLoginKey,
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

  it("honors a per-call maxFailures override (global bucket: 20 failures)", () => {
    const rl = new LoginRateLimiter();
    for (let i = 0; i < GLOBAL_MAX_FAILURES - 1; i++) {
      const res = rl.recordFailure(GLOBAL_LIMIT_KEY, T0 + i, GLOBAL_MAX_FAILURES);
      expect(res.allowed).toBe(true);
      expect(res.remaining).toBe(GLOBAL_MAX_FAILURES - i - 1);
    }
    const last = rl.recordFailure(GLOBAL_LIMIT_KEY, T0 + 19, GLOBAL_MAX_FAILURES);
    expect(last.allowed).toBe(false);
    expect(
      rl.check(GLOBAL_LIMIT_KEY, T0 + 20, GLOBAL_MAX_FAILURES).allowed,
    ).toBe(false);
    expect(
      rl.check(GLOBAL_LIMIT_KEY, T0 + 19 + DEFAULT_LOCKOUT_MS, GLOBAL_MAX_FAILURES)
        .allowed,
    ).toBe(true);
  });
});

describe("resolveLoginKey", () => {
  it("ignores forwarding headers when the proxy is untrusted (default)", () => {
    const resolved = resolveLoginKey({
      trustProxyHeaders: false,
      forwardedFor: "6.6.6.6",
      realIp: "7.7.7.7",
    });
    expect(resolved).toEqual({
      key: GLOBAL_LIMIT_KEY,
      maxFailures: GLOBAL_MAX_FAILURES,
    });
  });

  it("uses the same global bucket for every caller when untrusted", () => {
    const a = resolveLoginKey({
      trustProxyHeaders: false,
      forwardedFor: "1.1.1.1",
      realIp: null,
    });
    const b = resolveLoginKey({
      trustProxyHeaders: false,
      forwardedFor: "2.2.2.2",
      realIp: null,
    });
    expect(a.key).toBe(b.key);
  });

  it("keys on the first x-forwarded-for hop when trusted", () => {
    const resolved = resolveLoginKey({
      trustProxyHeaders: true,
      forwardedFor: "203.0.113.7, 10.0.0.1",
      realIp: "10.0.0.1",
    });
    expect(resolved).toEqual({
      key: "ip:203.0.113.7",
      maxFailures: DEFAULT_MAX_FAILURES,
    });
  });

  it("falls back to x-real-ip, then 'unknown', when trusted", () => {
    expect(
      resolveLoginKey({
        trustProxyHeaders: true,
        forwardedFor: null,
        realIp: "198.51.100.1",
      }).key,
    ).toBe("ip:198.51.100.1");
    expect(
      resolveLoginKey({
        trustProxyHeaders: true,
        forwardedFor: "  ",
        realIp: null,
      }).key,
    ).toBe("ip:unknown");
  });
});

describe("LoginRateLimiter eviction sweep", () => {
  it("does not sweep while under the cap", () => {
    const rl = new LoginRateLimiter({ maxEntries: 10, lockoutMs: 100 });
    rl.recordFailure("a", T0);
    rl.recordFailure("b", T0);
    rl.check("c", T0 + 10_000);
    expect(rl.size()).toBe(2);
  });

  it("sweeps idle non-locked entries once over the cap", () => {
    const rl = new LoginRateLimiter({ maxEntries: 2, lockoutMs: 100 });
    rl.recordFailure("a", T0);
    rl.recordFailure("b", T0);
    // Third key pushes the map over the cap; a and b are idle past the window.
    rl.recordFailure("c", T0 + 200);
    expect(rl.size()).toBe(1);
    expect(rl.check("c", T0 + 200).remaining).toBe(DEFAULT_MAX_FAILURES - 1);
  });

  it("sweeps expired locks but keeps active locks", () => {
    const rl = new LoginRateLimiter({
      maxEntries: 2,
      maxFailures: 1,
      lockoutMs: 100,
    });
    rl.recordFailure("expired", T0); // locked until T0+100
    rl.recordFailure("active", T0 + 150); // locked until T0+250
    rl.recordFailure("fresh", T0 + 200); // over cap -> sweep at T0+200
    expect(rl.check("active", T0 + 200).allowed).toBe(false);
    expect(rl.size()).toBe(2); // "expired" swept, "active" + "fresh" kept
  });

  it("keeps recently active entries during a sweep", () => {
    const rl = new LoginRateLimiter({ maxEntries: 1, lockoutMs: 100 });
    rl.recordFailure("recent", T0);
    rl.recordFailure("new", T0 + 50); // over cap, but "recent" is within window
    expect(rl.size()).toBe(2);
  });
});
