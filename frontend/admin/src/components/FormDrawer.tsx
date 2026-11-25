import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Button, Drawer, Modal, Space, Tooltip, type FormInstance } from 'antd';
import { Maximize2, Minimize2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { FORM_DRAWER_WIDTH } from '@/constants/layout';
import { usePreferencesStore } from '@/stores/preferences';

export interface FormDrawerProps {
  cancelText?: string;
  children: ReactNode;
  confirmLoading?: boolean;
  /**
   * Form instance backing the drawer. Supplying it turns on the unsaved-changes
   * guard: closing with edits in progress asks first instead of throwing them
   * away. Without it the drawer cannot tell an edited form from an untouched
   * one, so it closes silently.
   */
  form?: FormInstance;
  /** Explicit dirty flag, for drawers whose form is not a single antd Form. */
  dirty?: boolean;
  okText?: string;
  onClose: () => void;
  onOk: () => void | Promise<void>;
  open: boolean;
  /** Unmount the body while closed. Defaults to true so every open remounts its form. */
  destroyOnHidden?: boolean;
  title: ReactNode;
  /** Restored (non-maximized) width. Defaults to 576px. */
  width?: number;
}

/**
 * Create/edit form side drawer: maximize in the header, no X close,
 * sticky Cancel + Save footer. Closing via Cancel or mask.
 *
 * Every way out of the drawer — Cancel, mask, Esc, browser refresh — runs
 * through the same guard, so a half-filled form cannot disappear by accident.
 */
export function FormDrawer({
  cancelText,
  children,
  confirmLoading = false,
  destroyOnHidden = true,
  dirty,
  form,
  okText,
  onClose,
  onOk,
  open,
  title,
  width = FORM_DRAWER_WIDTH,
}: FormDrawerProps) {
  const { t } = useTranslation();
  const cancelLabel = cancelText ?? t('common.cancel');
  const okLabel = okText ?? t('common.save');
  const [maximized, setMaximized] = useState(false);
  const maximizeEnabled = usePreferencesStore((state) => state.preferences.custom.enableFormFullscreen);

  useEffect(() => {
    if (!open) setMaximized(false);
  }, [open]);

  // Read on demand rather than rendered: isFieldsTouched() is not reactive, and
  // a value captured at render time would be one keystroke stale at close time.
  const hasUnsavedChanges = () => {
    if (dirty !== undefined) return dirty;
    return form ? form.isFieldsTouched() : false;
  };
  // The unload handler outlives a render, so it reads the latest check through
  // a ref instead of a captured closure.
  const unsavedRef = useRef(hasUnsavedChanges);
  unsavedRef.current = hasUnsavedChanges;

  // A refresh or a closed tab bypasses the drawer entirely.
  useEffect(() => {
    if (!open) return;
    const handler = (event: BeforeUnloadEvent) => {
      if (!unsavedRef.current()) return;
      event.preventDefault();
      // Chrome ignores a custom string but still needs returnValue set.
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [open]);

  const confirmDiscard = () =>
    new Promise<boolean>((resolve) => {
      Modal.confirm({
        title: t('common.unsavedTitle'),
        content: t('common.unsavedDetail'),
        okText: t('common.unsavedLeave'),
        okButtonProps: { danger: true },
        cancelText: t('common.unsavedStay'),
        onOk: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });

  const handleClose = async () => {
    if (hasUnsavedChanges()) {
      const leave = await confirmDiscard();
      if (!leave) return;
    }
    setMaximized(false);
    onClose();
  };

  const maximizeLabel = maximized ? t('tabbar.restore') : t('tabbar.maximize');

  return (
    <Drawer
      className="form-drawer"
      closable={false}
      destroyOnHidden={destroyOnHidden}
      extra={
        maximizeEnabled ? (
          <Tooltip title={maximizeLabel}>
            <Button
              aria-label={maximizeLabel}
              icon={maximized ? <Minimize2 size={15} /> : <Maximize2 size={15} />}
              type="text"
              onClick={() => setMaximized((value) => !value)}
            />
          </Tooltip>
        ) : null
      }
      footer={
        <div className="form-drawer-footer">
          <Space>
            <Button onClick={() => void handleClose()}>{cancelLabel}</Button>
            <Button loading={confirmLoading} type="primary" onClick={() => void onOk()}>
              {okLabel}
            </Button>
          </Space>
        </div>
      }
      mask={{ closable: true }}
      onClose={() => void handleClose()}
      open={open}
      placement="right"
      rootClassName={maximized ? 'form-drawer-maximized' : undefined}
      size={maximized ? '100vw' : width}
      title={title}
    >
      {children}
    </Drawer>
  );
}
