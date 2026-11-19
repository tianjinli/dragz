import { Modal, Typography, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { useTranslation } from 'react-i18next';

const SIZE = 256;

interface AvatarCropModalProps {
  open: boolean;
  file?: File | null;
  onCancel: () => void;
  onSave: (blob: Blob) => Promise<void> | void;
}

interface LoadedImage {
  element: HTMLImageElement;
  url: string;
  baseScale: number;
}

interface Offset {
  x: number;
  y: number;
}

/** Draw the current crop into any square canvas using the same cover transform as the viewport. */
function drawCrop(
  context: CanvasRenderingContext2D,
  loaded: LoadedImage,
  zoom: number,
  offset: Offset,
  size: number,
) {
  const ratio = size / SIZE;
  const scale = loaded.baseScale * zoom * ratio;
  const { element } = loaded;
  context.fillStyle = '#ffffff';
  context.fillRect(0, 0, size, size);
  context.drawImage(
    element,
    size / 2 + offset.x * ratio - (element.naturalWidth * scale) / 2,
    size / 2 + offset.y * ratio - (element.naturalHeight * scale) / 2,
    element.naturalWidth * scale,
    element.naturalHeight * scale,
  );
}

/**
 * WeChat-style avatar cropper: a circular crop frame with dimmed surroundings,
 * drag-to-pan and mouse-wheel zoom.
 */
export function AvatarCropModal({ open, file, onCancel, onSave }: AvatarCropModalProps) {
  const { t } = useTranslation();
  const [loaded, setLoaded] = useState<LoadedImage | null>(null);
  const [zoom, setZoom] = useState(1);
  const [offset, setOffset] = useState<Offset>({ x: 0, y: 0 });
  const [saving, setSaving] = useState(false);
  const drag = useRef<{ pointerX: number; pointerY: number; startX: number; startY: number } | null>(null);
  const viewportRef = useRef<HTMLDivElement>(null);

  const clamp = useCallback(
    (next: Offset, nextZoom: number, image: LoadedImage): Offset => {
      const width = image.element.naturalWidth * image.baseScale * nextZoom;
      const height = image.element.naturalHeight * image.baseScale * nextZoom;
      const maxX = Math.max(0, (width - SIZE) / 2);
      const maxY = Math.max(0, (height - SIZE) / 2);
      return {
        x: Math.min(maxX, Math.max(-maxX, next.x)),
        y: Math.min(maxY, Math.max(-maxY, next.y)),
      };
    },
    [],
  );

  useEffect(() => {
    const node = viewportRef.current;
    if (!node || !loaded) return;
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      const nextZoom = Math.min(3, Math.max(1, zoom - event.deltaY * 0.002));
      setZoom(nextZoom);
      setOffset((previous) => clamp(previous, nextZoom, loaded));
    };
    node.addEventListener('wheel', onWheel, { passive: false });
    return () => node.removeEventListener('wheel', onWheel);
  }, [clamp, loaded, zoom]);

  useEffect(() => {
    if (!open || !file) return;
    let cancelled = false;
    let objectUrl = '';
    const element = new Image();

    setLoaded(null);
    objectUrl = URL.createObjectURL(file);
    element.onload = () => {
      if (cancelled) return;
      const baseScale = Math.max(SIZE / element.naturalWidth, SIZE / element.naturalHeight);
      setLoaded({ element, url: objectUrl, baseScale });
      setZoom(1);
      setOffset({ x: 0, y: 0 });
    };
    element.onerror = () => {
      if (!cancelled) void message.error(t('upload.readImageFailed'));
      URL.revokeObjectURL(objectUrl);
    };
    element.src = objectUrl;

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
      setLoaded(null);
    };
  }, [file, open, t]);

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!loaded) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = {
      pointerX: event.clientX,
      pointerY: event.clientY,
      startX: offset.x,
      startY: offset.y,
    };
  };

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!loaded || !drag.current) return;
    const next = {
      x: drag.current.startX + event.clientX - drag.current.pointerX,
      y: drag.current.startY + event.clientY - drag.current.pointerY,
    };
    setOffset(clamp(next, zoom, loaded));
  };

  const onPointerUp = () => {
    drag.current = null;
  };

  const handleSave = async () => {
    if (!loaded) return;
    setSaving(true);
    try {
      const canvas = document.createElement('canvas');
      canvas.width = SIZE;
      canvas.height = SIZE;
      const context = canvas.getContext('2d');
      if (!context) throw new Error('canvas unavailable');
      drawCrop(context, loaded, zoom, offset, SIZE);
      const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.9));
      if (!blob) throw new Error('encode failed');
      await onSave(blob);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      cancelText={t('common.cancel')}
      confirmLoading={saving}
      okText={t('common.save')}
      onCancel={onCancel}
      onOk={() => void handleSave()}
      open={open}
      title={t('shell.avatarCropTitle')}
    >
      <div className="avatar-crop">
        <div
          className="avatar-crop-viewport"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          ref={viewportRef}
        >
          {loaded ? (
            <img
              alt=""
              draggable={false}
              src={loaded.url}
              style={{
                width: loaded.element.naturalWidth * loaded.baseScale * zoom,
                height: loaded.element.naturalHeight * loaded.baseScale * zoom,
                transform: `translate(calc(-50% + ${offset.x}px), calc(-50% + ${offset.y}px))`,
              }}
            />
          ) : null}
          <div aria-hidden="true" className="avatar-crop-mask" />
          <div aria-hidden="true" className="avatar-crop-ring" />
        </div>
        <Typography.Text type="secondary">{t('shell.avatarCropHint')}</Typography.Text>
      </div>
    </Modal>
  );
}
