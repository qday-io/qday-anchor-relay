import { describe, expect, it } from "vitest";
import { decodeMemo, encodeMemo, MemoError } from "./memo";

const ZEROS = "0".repeat(64);
const EFFS = "f".repeat(64);

// Golden vectors shared with the Go relay implementation. Do not alter.
const GOLDEN = [
  {
    batch: 1024n,
    root: "3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e",
    memo: "QDAY2ANCHOR|v1|1024|0x3fa8b1c2d3e4f5061728394a5b6c7d8e9fa0b1c2d3e4f50617283946a5b6c21e",
  },
  {
    batch: 0n,
    root: ZEROS,
    memo: `QDAY2ANCHOR|v1|0|0x${ZEROS}`,
  },
  {
    batch: 18446744073709551615n,
    root: EFFS,
    memo: `QDAY2ANCHOR|v1|18446744073709551615|0x${EFFS}`,
  },
] as const;

describe("encodeMemo", () => {
  it.each(GOLDEN)("golden vector batch=$batch", ({ batch, root, memo }) => {
    expect(encodeMemo(batch, root)).toBe(memo);
  });

  it("accepts a 0x-prefixed state root", () => {
    expect(encodeMemo(1024n, `0x${GOLDEN[0].root}`)).toBe(GOLDEN[0].memo);
  });

  it("rejects a negative batch number", () => {
    expect(() => encodeMemo(-1n, ZEROS)).toThrow(MemoError);
  });

  it("rejects a batch number above uint64 max", () => {
    expect(() => encodeMemo(18446744073709551616n, ZEROS)).toThrow(MemoError);
  });

  it("rejects uppercase hex in the state root", () => {
    expect(() => encodeMemo(1n, ZEROS.slice(1) + "A")).toThrow(MemoError);
  });

  it("rejects a state root that is not 64 hex chars", () => {
    expect(() => encodeMemo(1n, "abcd")).toThrow(MemoError);
    expect(() => encodeMemo(1n, ZEROS + "0")).toThrow(MemoError);
    expect(() => encodeMemo(1n, "")).toThrow(MemoError);
  });
});

describe("decodeMemo", () => {
  it.each(GOLDEN)("round-trips golden vector batch=$batch", ({ batch, root, memo }) => {
    const decoded = decodeMemo(memo);
    expect(decoded.batchNumber).toBe(batch);
    expect(decoded.stateRoot).toBe(`0x${root}`);
  });

  it("rejects the empty string", () => {
    expect(() => decodeMemo("")).toThrow(MemoError);
  });

  it("rejects a wrong prefix", () => {
    expect(() => decodeMemo(`QDAY2ANCHOX|v1|1|0x${ZEROS}`)).toThrow(MemoError);
    expect(() => decodeMemo(`qday2anchor|v1|1|0x${ZEROS}`)).toThrow(MemoError);
  });

  it("rejects a wrong version", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v2|1|0x${ZEROS}`)).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|V1|1|0x${ZEROS}`)).toThrow(MemoError);
  });

  it("rejects when there are not exactly 4 segments", () => {
    expect(() => decodeMemo("QDAY2ANCHOR|v1|1")).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|0x${ZEROS}|extra`)).toThrow(
      MemoError,
    );
    expect(() => decodeMemo("QDAY2ANCHOR")).toThrow(MemoError);
  });

  it("rejects a state root without the 0x prefix", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|${ZEROS}`)).toThrow(MemoError);
  });

  it("rejects a state root that is not exactly 64 hex chars", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|0x${ZEROS.slice(1)}`)).toThrow(
      MemoError,
    );
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|0x${ZEROS}0`)).toThrow(MemoError);
    expect(() => decodeMemo("QDAY2ANCHOR|v1|1|0x")).toThrow(MemoError);
  });

  it("rejects uppercase hex in the state root", () => {
    expect(() =>
      decodeMemo(`QDAY2ANCHOR|v1|1|0x${ZEROS.slice(1)}A`),
    ).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|0x${EFFS.toUpperCase()}`)).toThrow(
      MemoError,
    );
  });

  it("rejects non-hex characters in the state root", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1|0x${ZEROS.slice(1)}g`)).toThrow(
      MemoError,
    );
  });

  it("rejects a non-numeric batch number", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|abc|0x${ZEROS}`)).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|1.5|0x${ZEROS}`)).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|v1||0x${ZEROS}`)).toThrow(MemoError);
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|0x10|0x${ZEROS}`)).toThrow(MemoError);
  });

  it("rejects a negative batch number", () => {
    expect(() => decodeMemo(`QDAY2ANCHOR|v1|-1|0x${ZEROS}`)).toThrow(MemoError);
  });

  it("rejects a batch number above uint64 max", () => {
    expect(() =>
      decodeMemo(`QDAY2ANCHOR|v1|18446744073709551616|0x${ZEROS}`),
    ).toThrow(MemoError);
  });
});
