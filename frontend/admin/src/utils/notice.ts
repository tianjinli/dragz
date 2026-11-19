export function normalizeNoticePriority(value: unknown): 'normal' | 'important' | 'urgent' {
  const raw = String(value || 'normal');
  if (raw === 'important' || raw === 'urgent') return raw;
  return 'normal';
}

/** Canonical message type stored on sys_notice.notice_type. */
export function normalizeNoticeType(value: unknown): 'announcement' | 'notification' {
  return String(value || '') === 'notification' ? 'notification' : 'announcement';
}

export function isAnnouncementNoticeType(value: unknown) {
  return normalizeNoticeType(value) === 'announcement';
}
