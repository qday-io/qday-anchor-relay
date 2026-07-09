import { describe, expect, it } from "vitest";
import { resolveRelayAdminUrl } from "./relay-admin";

describe("resolveRelayAdminUrl", () => {
  it("uses RELAY_ADMIN_URL when set", () => {
    expect(resolveRelayAdminUrl("http://relay-admin:9000", "production")).toBe(
      "http://relay-admin:9000",
    );
    expect(resolveRelayAdminUrl("http://10.0.0.5:8081", "development")).toBe(
      "http://10.0.0.5:8081",
    );
  });

  it("strips trailing slashes from the configured URL", () => {
    expect(resolveRelayAdminUrl("http://relay:8081/", "production")).toBe(
      "http://relay:8081",
    );
    expect(resolveRelayAdminUrl("http://relay:8081///", "production")).toBe(
      "http://relay:8081",
    );
  });

  it("defaults to the compose service name in production", () => {
    expect(resolveRelayAdminUrl(undefined, "production")).toBe(
      "http://relay:8081",
    );
  });

  it("falls back to localhost in local dev", () => {
    expect(resolveRelayAdminUrl(undefined, "development")).toBe(
      "http://localhost:8081",
    );
    expect(resolveRelayAdminUrl(undefined, undefined)).toBe(
      "http://localhost:8081",
    );
  });

  it("treats empty/whitespace values as unset", () => {
    expect(resolveRelayAdminUrl("", "production")).toBe("http://relay:8081");
    expect(resolveRelayAdminUrl("   ", "development")).toBe(
      "http://localhost:8081",
    );
  });
});
