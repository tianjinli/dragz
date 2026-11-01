import {
  COMPRESS_PRESETS,
  fitMaxEdge,
  resolveCompressPreset,
  type CompressPreset,
} from './compress-presets';

export type { CompressPreset } from './compress-presets';
export { COMPRESS_PRESETS, fitMaxEdge, resolveCompressPreset } from './compress-presets';

function loadImage(file: Blob): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('failed to decode image'));
    };
    img.src = url;
  });
}

function canvasToJpegBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) resolve(blob);
        else reject(new Error('canvas toBlob failed'));
      },
      'image/jpeg',
      quality,
    );
  });
}

/**
 * Compress an image File/Blob for upload. Returns a JPEG File when compressed,
 * or the original input when already small / non-image / on failure.
 */
export async function compressImageFile(
  input: Blob | File,
  preset?: CompressPreset | string | null,
): Promise<File> {
  const name =
    input instanceof File && input.name
      ? input.name.replace(/\.[^.]+$/, '') || 'image'
      : 'image';
  const cfg = COMPRESS_PRESETS[resolveCompressPreset(preset)];
  const type = String(input.type || '').toLowerCase();
  if (type && !type.startsWith('image/')) {
    return input instanceof File ? input : new File([input], `${name}.bin`, { type });
  }

  try {
    const img = await loadImage(input);
    const longest = Math.max(img.naturalWidth, img.naturalHeight);
    if (input.size > 0 && input.size <= cfg.skipBelowBytes && longest <= cfg.maxEdge) {
      return input instanceof File
        ? input
        : new File([input], `${name}.jpg`, { type: input.type || 'image/jpeg' });
    }

    const { width, height } = fitMaxEdge(img.naturalWidth, img.naturalHeight, cfg.maxEdge);
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('no 2d context');
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(img, 0, 0, width, height);
    const blob = await canvasToJpegBlob(canvas, cfg.quality);
    return new File([blob], `${name}.jpg`, { type: 'image/jpeg', lastModified: Date.now() });
  } catch {
    return input instanceof File
      ? input
      : new File([input], `${name}.bin`, { type: input.type || 'application/octet-stream' });
  }
}
