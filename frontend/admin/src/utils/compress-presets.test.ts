import { describe, expect, it } from 'vitest';
import { COMPRESS_PRESETS, fitMaxEdge, resolveCompressPreset } from './compress-presets';

describe('compress presets', () => {
  it('resolves known and default presets', () => {
    expect(resolveCompressPreset('icon')).toBe('icon');
    expect(resolveCompressPreset('cover')).toBe('cover');
    expect(resolveCompressPreset('')).toBe('thumb');
    expect(resolveCompressPreset('nope')).toBe('thumb');
  });

  it('fits max edge keeping aspect ratio', () => {
    expect(fitMaxEdge(4000, 3000, 960)).toEqual({ width: 960, height: 720 });
    expect(fitMaxEdge(800, 600, 960)).toEqual({ width: 800, height: 600 });
    expect(fitMaxEdge(256, 256, 256)).toEqual({ width: 256, height: 256 });
  });

  it('has expected max edges from product table', () => {
    expect(COMPRESS_PRESETS.icon.maxEdge).toBe(256);
    expect(COMPRESS_PRESETS.thumb.maxEdge).toBe(640);
    expect(COMPRESS_PRESETS.cover.maxEdge).toBe(960);
    expect(COMPRESS_PRESETS.banner.maxEdge).toBe(960);
    expect(COMPRESS_PRESETS.evidence.maxEdge).toBe(1280);
    expect(COMPRESS_PRESETS.photo.maxEdge).toBe(1920);
  });
});
