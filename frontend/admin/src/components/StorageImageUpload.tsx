import { useEffect, useState, type DragEvent, type ReactNode } from "react";
import { Image, Popover, Upload, message } from "antd";
import { LoadingOutlined } from "@ant-design/icons";
import { Plus, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { requestClient } from "@/api/request";
import {
  parseStorageFileId,
  storageFileContentUrl,
  toAbsoluteHttpUrl,
  uploadStorageFileApi,
} from "@/api/system";

function isAbsoluteHttpUrl(value: string) {
  return /^https?:\/\//i.test(value);
}

export type StorageImagePreset =
  | "icon"
  | "thumb"
  | "cover"
  | "banner"
  | "evidence"
  | "photo";

/** Resolve a stored cover/url value to a public absolute http(s) content URL when possible. */
export function resolveStorageImageSrc(urlOrId?: string | number | null) {
  if (urlOrId == null || urlOrId === "") return "";
  const raw = String(urlOrId).trim();
  // 客服二维码这类内联图本身就是 data URI，绝对化会把整串拼坏
  if (/^(data|blob):/i.test(raw)) return raw;
  const fileId = parseStorageFileId(urlOrId);
  if (fileId) return storageFileContentUrl(fileId);
  return toAbsoluteHttpUrl(raw);
}

/** Admin preview: prefer authenticated download blob so private/public both echo. */
function useStoragePreviewSrc(urlOrId?: string | number | null) {
  const [src, setSrc] = useState("");
  const fileId = parseStorageFileId(urlOrId);
  const fallback = resolveStorageImageSrc(urlOrId);

  useEffect(() => {
    let objectUrl = "";
    let cancelled = false;

    if (fileId) {
      setSrc("");
      void requestClient
        .get<Blob>(`/system/files/${fileId}/download`, { responseType: "blob" })
        .then((response) => {
          if (cancelled) return;
          objectUrl = URL.createObjectURL(response.data);
          setSrc(objectUrl);
        })
        .catch(() => {
          if (!cancelled) setSrc(fallback);
        });
    } else {
      setSrc(fallback);
    }

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [fileId, fallback]);

  return src;
}

export function StorageImageThumb({
  url,
  size = 48,
  alt,
}: {
  url?: string | number | null;
  size?: number;
  alt?: string;
}) {
  const { t } = useTranslation();
  const src = useStoragePreviewSrc(url);
  if (!url) return <span>—</span>;
  if (!src) return <span style={{ fontSize: 12, color: "#999" }}>…</span>;
  return (
    <Popover
      trigger="hover"
      placement="right"
      mouseEnterDelay={0.15}
      content={
        <Image
          alt={t("common.preview")}
          src={src}
          width={280}
          style={{ maxHeight: 280, objectFit: "contain", display: "block" }}
          preview={false}
        />
      }
    >
      <Image
        alt={alt ?? t("common.image")}
        src={src}
        width={size}
        height={size}
        style={{
          objectFit: "cover",
          borderRadius: 4,
          cursor: "zoom-in",
          display: "block",
        }}
        preview={false}
      />
    </Popover>
  );
}

async function fileToDataURI(file: Blob, preset: StorageImagePreset, errorMessage: string): Promise<string> {
  const { compressImageFile } = await import("@/utils/compress-image");
  const compressed = await compressImageFile(file, preset);
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(new Error(errorMessage));
    reader.readAsDataURL(compressed);
  });
}

/**
 * Form-friendly image upload bound to system storage (`/system/files/upload`).
 * Value is the public content URL returned by the storage API (or empty string).
 * Supports click-to-upload and dragging a file onto the tile to add or replace.
 */
export function StorageImageUpload({
  value,
  onChange,
  accept = "image/*",
  preset = "thumb",
  compress = true,
  mode = "storage",
}: {
  value?: string;
  onChange?: (url: string) => void;
  accept?: string;
  /** Compression preset; default thumb for list-sized assets. */
  preset?: StorageImagePreset;
  /** When false, upload the original file bytes (no resize/JPEG re-encode). */
  compress?: boolean;
  /** `dataUri` keeps the image inline (customer-service QR); `storage` uploads it. */
  mode?: "storage" | "dataUri";
}) {
  const { t } = useTranslation();
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const src = useStoragePreviewSrc(value);

  const submit = (file: Blob, done?: (res: unknown) => void, fail?: (err: Error) => void) => {
    setUploading(true);
    void (async () => {
      try {
        if (mode === "dataUri") {
          onChange?.(await fileToDataURI(file, preset, t("upload.readImageFailed")));
          done?.({});
          void message.success(t("upload.imageUpdated"));
          return;
        }
        let uploadBlob: Blob | File = file;
        if (compress) {
          const { compressImageFile } = await import("@/utils/compress-image");
          uploadBlob = await compressImageFile(file, preset);
        }
        const item = await uploadStorageFileApi(uploadBlob, { accessType: "public" });
        const id = Number(item.id);
        const raw =
          (typeof item.url === "string" && item.url) ||
          (Number.isFinite(id) && id > 0 ? storageFileContentUrl(id) : "");
        const url = toAbsoluteHttpUrl(raw);
        if (!url || !isAbsoluteHttpUrl(url)) {
          const invalid = new Error(t("upload.absoluteUrlRequired"));
          invalid.name = "StorageImageUrlError";
          throw invalid;
        }
        onChange?.(url);
        done?.({});
        void message.success(t("upload.uploadSuccess"));
      } catch (error) {
        const err = error instanceof Error ? error : new Error(t("upload.uploadFailed"));
        fail?.(err);
        void message.error(err.name === "StorageImageUrlError" ? err.message : t("upload.uploadFailed"));
      } finally {
        setUploading(false);
      }
    })();
  };

  const droppedFile = (event: DragEvent<HTMLDivElement>) => {
    const file = event.dataTransfer?.files?.[0];
    if (!file) return undefined;
    // 拖进来的文件不走 accept，自己挡一道非图片
    if (accept.includes("image") && !file.type.startsWith("image/")) {
      void message.error(t("upload.imageOnly"));
      return undefined;
    }
    return file;
  };

  return (
    <div
      className={`storage-image-upload${dragOver ? " storage-image-upload--over" : ""}`}
      onDragOver={(event) => {
        event.preventDefault();
        event.stopPropagation();
        if (!dragOver) setDragOver(true);
      }}
      onDragLeave={(event) => {
        event.preventDefault();
        setDragOver(false);
      }}
      onDrop={(event) => {
        event.preventDefault();
        event.stopPropagation();
        setDragOver(false);
        const file = droppedFile(event);
        if (file) submit(file);
      }}
    >
      <Upload
        accept={accept}
        listType="picture-card"
        showUploadList={false}
        maxCount={1}
        customRequest={({ file, onSuccess, onError }) => {
          submit(file as Blob, onSuccess, onError);
        }}
      >
        {src ? (
          <img
            alt={t("common.preview")}
            src={src}
            style={{ width: "100%", height: "100%", objectFit: "cover" }}
          />
        ) : (
          <div>
            {uploading ? <LoadingOutlined /> : <Plus size={18} />}
            <div style={{ marginTop: 8 }}>
              {uploading ? t("common.uploading") : value ? t("common.loading") : t("common.upload")}
            </div>
          </div>
        )}
      </Upload>
      {value ? (
        <button
          type="button"
          className="storage-image-upload-remove"
          aria-label={t("upload.removeImage")}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onChange?.("");
          }}
        >
          <X size={12} strokeWidth={2.5} />
        </button>
      ) : null}
    </div>
  );
}

/** fileId-based preview (home banners). */
export function StorageFileImage({
  fileId,
  width = 72,
  height,
  alt,
  empty = <span>—</span>,
}: {
  fileId?: number;
  width?: number;
  height?: number;
  alt?: string;
  empty?: ReactNode;
}) {
  const { t } = useTranslation();
  const src = useStoragePreviewSrc(fileId);
  if (!fileId) return <>{empty}</>;
  if (!src) return <span style={{ fontSize: 12, color: "#999" }}>{t("common.loading")}</span>;
  return (
    <Image
      alt={alt ?? t("common.image")}
      src={src}
      width={width}
      height={height ?? Math.round(width * 0.5)}
      style={{ objectFit: "cover", borderRadius: 4 }}
    />
  );
}
