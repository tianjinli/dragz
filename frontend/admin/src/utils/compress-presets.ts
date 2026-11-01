/** Shared image upload compression presets (admin + wxapp semantics). */

export type CompressPreset =
  | 'icon'
  | 'thumb'
  | 'cover'
  | 'banner'
  | 'evidence'
  | 'photo';

export type CompressPresetConfig = {
  maxEdge: number;
  quality: number;
  /** Skip re-encode when already small enough (bytes). */
  skipBelowBytes: number;
};

export const COMPRESS_PRESETS: Record<CompressPreset, CompressPresetConfig> = {
  icon: { maxEdge: 256, quality: 0.75, skipBelowBytes: 200 * 1024 },
  thumb: { maxEdge: 640, quality: 0.75, skipBelowBytes: 200 * 1024 },
  cover: { maxEdge: 960, quality: 0.8, skipBelowBytes: 200 * 1024 },
  banner: { maxEdge: 960, quality: 0.8, skipBelowBytes: 200 * 1024 },
  evidence: { maxEdge: 1280, quality: 0.75, skipBelowBytes: 300 * 1024 },
  photo: { maxEdge: 1920, quality: 0.8, skipBelowBytes: 300 * 1024 },
};

export function resolveCompressPreset(preset?: string | null): CompressPreset {
  const key = String(preset || '').trim() as CompressPreset;
  if (key && key in COMPRESS_PRESETS) return key;
  return 'thumb';
}

/** Scale so longest edge <= maxEdge; keep aspect ratio. */
export function fitMaxEdge(width: number, height: number, maxEdge: number) {
  const w = Math.max(1, Number(width) || 1);
  const h = Math.max(1, Number(height) || 1);
  const edge = Math.max(1, Number(maxEdge) || 1);
  const longest = Math.max(w, h);
  if (longest <= edge) return { width: Math.round(w), height: Math.round(h) };
  const scale = edge / longest;
  return {
    width: Math.max(1, Math.round(w * scale)),
    height: Math.max(1, Math.round(h * scale)),
  };
}
