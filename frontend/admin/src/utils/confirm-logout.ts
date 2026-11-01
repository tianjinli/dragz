import { Modal } from 'antd';
import i18n from '@/i18n';

/** Centered logout confirm; Cancel is focused by default. */
export function confirmLogout(onOk: () => void | Promise<void>, title = i18n.t('auth.logoutConfirm')) {
  Modal.confirm({
    title,
    centered: true,
    okText: i18n.t('common.confirm'),
    cancelText: i18n.t('common.cancel'),
    autoFocusButton: 'cancel',
    // Do not return the async work — Ant Design keeps the dialog open while the
    // returned Promise is pending, and navigation during logout can leave it stuck.
    onOk: () => {
      void Promise.resolve(onOk()).finally(() => {
        Modal.destroyAll();
      });
    },
  });
}
