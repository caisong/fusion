import { describe, expect, it } from "vitest";
import { normalizeBasePath } from "./base-path";

describe("normalizeBasePath", () => {
  it("treats the domain root as an empty prefix", () => {
    expect(normalizeBasePath("")).toBe("");
    expect(normalizeBasePath("/")).toBe("");
    expect(normalizeBasePath("  /  ")).toBe("");
  });

  it("keeps a single leading slash and drops trailing slashes", () => {
    expect(normalizeBasePath("/fusion")).toBe("/fusion");
    expect(normalizeBasePath("/fusion/")).toBe("/fusion");
    expect(normalizeBasePath("fusion")).toBe("/fusion");
    expect(normalizeBasePath("//fusion//")).toBe("/fusion");
  });

  it("preserves nested prefixes", () => {
    expect(normalizeBasePath("/apps/fusion/")).toBe("/apps/fusion");
  });
});
