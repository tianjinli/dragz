import { describe, expect, it } from "vitest";
import {
  parseStorageFileId,
  storageFileContentUrl,
  toAbsoluteHttpUrl,
} from "./system";

describe("toAbsoluteHttpUrl", () => {
  it("keeps absolute http(s) urls", () => {
    expect(toAbsoluteHttpUrl("https://cdn.example/a.png")).toBe(
      "https://cdn.example/a.png",
    );
    expect(toAbsoluteHttpUrl("http://localhost:8888/api/system/files/1/content")).toBe(
      "http://localhost:8888/api/system/files/1/content",
    );
  });

  it("prefixes relative paths with page origin", () => {
    const origin = window.location.origin;
    expect(toAbsoluteHttpUrl("/api/system/files/9/content")).toBe(
      `${origin}/api/system/files/9/content`,
    );
  });

  it("rejects empty", () => {
    expect(toAbsoluteHttpUrl("")).toBe("");
    expect(toAbsoluteHttpUrl(null)).toBe("");
  });
});

describe("storageFileContentUrl", () => {
  it("returns absolute http(s) content url", () => {
    const url = storageFileContentUrl(7);
    expect(url).toMatch(/^https?:\/\//);
    expect(url).toContain("/api/system/files/7/content");
  });
});

describe("parseStorageFileId", () => {
  it("parses absolute content urls", () => {
    expect(
      parseStorageFileId("https://admin.example/api/system/files/15/content"),
    ).toBe(15);
  });
});
