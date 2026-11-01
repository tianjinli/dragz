import { act, render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { StorageImageUpload } from "@/components/StorageImageUpload";

const compressImageFile = vi.fn(async (file: Blob) => file);
const uploadStorageFileApi = vi.fn(async () => ({
  id: 42,
  url: "https://cdn.example.com/files/42.png",
}));

vi.mock("@/utils/compress-image", () => ({
  compressImageFile: (...args: unknown[]) => compressImageFile(...args),
}));

vi.mock("@/api/system", () => ({
  parseStorageFileId: () => null,
  storageFileContentUrl: (id: number) => `https://cdn.example.com/files/${id}`,
  toAbsoluteHttpUrl: (url: string) => url,
  uploadStorageFileApi: (...args: unknown[]) => uploadStorageFileApi(...args),
}));

vi.mock("@/api/request", () => ({
  requestClient: { get: vi.fn() },
}));

// The static toast API mounts its own React root and schedules that render on the
// global timer queue. Under jsdom the render can land after the test file's
// environment is torn down, which surfaces as "window is not defined" from
// react-dom's scheduler and is reported as an unhandled error. These cases are
// about the upload flow, so the toast is stubbed out and schedules nothing.
vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    message: { ...actual.message, error: vi.fn(), success: vi.fn() },
  };
});

/** Let the upload promise chain and the state updates it triggers settle inside act. */
async function flushUpload() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

describe("StorageImageUpload", () => {
  beforeEach(() => {
    compressImageFile.mockClear();
    uploadStorageFileApi.mockClear();
  });

  it("skips compressImageFile when compress={false}", async () => {
    const onChange = vi.fn();
    const { container } = render(
      <StorageImageUpload compress={false} onChange={onChange} />,
    );

    const input = container.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    expect(input).toBeTruthy();

    const file = new File([new Uint8Array([1, 2, 3])], "bead.png", {
      type: "image/png",
    });
    Object.defineProperty(input, "files", { value: [file] });
    input.dispatchEvent(new Event("change", { bubbles: true }));

    await waitFor(() => {
      expect(uploadStorageFileApi).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(
        "https://cdn.example.com/files/42.png",
      );
    });
    expect(compressImageFile).not.toHaveBeenCalled();
    expect(uploadStorageFileApi.mock.calls[0]?.[0]).toBe(file);
    await flushUpload();
  });

  it("compresses before upload by default", async () => {
    const compressed = new File([new Uint8Array([9])], "bead.jpg", {
      type: "image/jpeg",
    });
    const onChange = vi.fn();
    compressImageFile.mockResolvedValueOnce(compressed);

    const { container } = render(
      <StorageImageUpload preset="cover" onChange={onChange} />,
    );
    const input = container.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    const file = new File([new Uint8Array([1])], "bead.png", {
      type: "image/png",
    });
    Object.defineProperty(input, "files", { value: [file] });
    input.dispatchEvent(new Event("change", { bubbles: true }));

    await waitFor(() => {
      expect(compressImageFile).toHaveBeenCalledTimes(1);
    });
    // Wait for the whole chain, not just the first call: the upload, the value
    // write-back and the "uploading" reset all land in later microtasks.
    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(
        "https://cdn.example.com/files/42.png",
      );
    });
    expect(uploadStorageFileApi).toHaveBeenCalledWith(compressed, {
      accessType: "public",
    });
    await flushUpload();
  });
});
