import { useEffect, useState, type ReactNode } from 'react';
import { Button, Drawer, Space, Tooltip } from 'antd';
import { Maximize2, Minimize2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { FORM_DRAWER_WIDTH } from '@/constants/layout';
import { usePreferencesStore } from '@/stores/preferences';

export interface FormDrawerProps {
  cancelText?: string;
  children: ReactNode;
  confirmLoading?: boolean;
  destroyOnClose?: boolean;
  okText?: string;
  onClose: () => void;
  onOk: () => void | Promise<void>;
  open: boolean;
  title: ReactNode;
  /** Restored (non-maximized) width. Defaults to 576px. */
  width?: number;
}

/**
 * Create/edit form side drawer: maximize in the header, no X close,
 * sticky Cancel + Save footer. Closing via Cancel or mask.
 */
export function FormDrawer({
  cancelText,
  children,
  confirmLoading = false,
  destroyOnClose = true,
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

  const handleClose = () => {
    setMaximized(false);
    onClose();
  };

  const maximizeLabel = maximized ? t('tabbar.restore') : t('tabbar.maximize');

  return (
    <Drawer
      className="form-drawer"
      closable={false}
      destroyOnClose={destroyOnClose}
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
            <Button onClick={handleClose}>{cancelLabel}</Button>
            <Button loading={confirmLoading} type="primary" onClick={() => void onOk()}>
              {okLabel}
            </Button>
          </Space>
        </div>
      }
      mask={{ closable: true }}
      onClose={handleClose}
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
