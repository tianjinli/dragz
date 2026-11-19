import { Tag, Tooltip } from 'antd';
import { useTranslation } from 'react-i18next';
import { normalizeNoticePriority } from '@/utils/notice';

export {
  isAnnouncementNoticeType,
  normalizeNoticePriority,
  normalizeNoticeType,
} from '@/utils/notice';

const PRIORITY_COLOR: Record<string, string> = {
  normal: 'default',
  important: 'warning',
  urgent: 'error',
};

const PRIORITY_LABEL: Record<string, string> = {
  normal: 'system.content.priorityNormal',
  important: 'system.content.priorityImportant',
  urgent: 'system.content.priorityUrgent',
};

const PRIORITY_PRESENT: Record<string, string> = {
  normal: 'system.content.priorityPresentNormal',
  important: 'system.content.priorityPresentImportant',
  urgent: 'system.content.priorityPresentUrgent',
};

/** Colored tag for 普通 / 重要 / 紧急, with presentation hint. */
export function NoticePriorityTag({ priority }: { priority?: unknown }) {
  const { t } = useTranslation();
  const value = normalizeNoticePriority(priority);
  return (
    <Tooltip title={t(PRIORITY_PRESENT[value])}>
      <Tag className={`notice-priority-tag is-${value}`} color={PRIORITY_COLOR[value]}>
        {t(PRIORITY_LABEL[value])}
      </Tag>
    </Tooltip>
  );
}
