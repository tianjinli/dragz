import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/system', () => ({
  listMyNoticesApi: vi.fn(async () => ({
    items: [
      {
        id: 1,
        title: '公告 A',
        noticeType: 'notice',
        read: false,
        publishAt: '2026-11-19T10:00:00Z',
      },
    ],
    total: 1,
  })),
  listMyNotificationsApi: vi.fn(async () => ({
    items: [
      {
        id: 2,
        title: '系统提醒',
        channel: 'system',
        content: 'hello',
        read: false,
        createdAt: '2026-11-19T11:00:00Z',
      },
    ],
    total: 1,
  })),
  markNoticeReadApi: vi.fn(async () => undefined),
  markNotificationReadApi: vi.fn(async () => undefined),
}));

vi.mock('@/stores/access', () => ({
  useAccessStore: {
    getState: () => ({ accessToken: 'token' }),
  },
}));

describe('useMessagesStore', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('merges notices and notifications and computes unread totals', async () => {
    const { useMessagesStore } = await import('./messages');
    useMessagesStore.setState({
      connected: false,
      items: [],
      unread: { notices: 0, notifications: 0, total: 0 },
      marqueeItems: [],
    });
    await useMessagesStore.getState().refresh();
    const state = useMessagesStore.getState();
    expect(state.items).toHaveLength(2);
    expect(state.unread.notices).toBe(1);
    expect(state.unread.notifications).toBe(1);
    expect(state.unread.total).toBe(2);
  });

  it('applies unread payloads from the socket', async () => {
    const { useMessagesStore } = await import('./messages');
    useMessagesStore.getState().applyUnread({
      notices: 3,
      notifications: 4,
      total: 7,
    });
    expect(useMessagesStore.getState().unread).toEqual({
      notices: 3,
      notifications: 4,
      total: 7,
    });
  });
});
