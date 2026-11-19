import { create } from "zustand";
import {
  listMyNoticesApi,
  listMyNotificationsApi,
  markNoticeReadApi,
  markNotificationReadApi,
  type ResourceRecord,
} from "@/api/system";
import { useAccessStore } from "@/stores/access";
import {
  ensureDesktopNotifyPermission,
  showDesktopNotify,
} from "@/utils/desktop-notify";
import {
  normalizeNoticePriority,
  normalizeNoticeType,
} from "@/utils/notice";

export type InboxKind = "notice" | "notification";

export interface InboxItem {
  id: number;
  kind: InboxKind;
  title: string;
  content?: string;
  summary?: string;
  channel?: string;
  noticeType?: string;
  priority?: string;
  read: boolean;
  at: string;
  raw?: ResourceRecord;
}

interface UnreadCounts {
  notices: number;
  notifications: number;
  total: number;
}

interface MessagesState {
  connected: boolean;
  items: InboxItem[];
  unread: UnreadCounts;
  /** Unread urgent notices shown as top marquee. */
  marqueeItems: InboxItem[];
  refresh: () => Promise<void>;
  markRead: (item: InboxItem) => Promise<void>;
  dismissMarquee: (id: number) => void;
  removeNotice: (id: number) => void;
  applyUnread: (counts: Partial<UnreadCounts>) => void;
  prepend: (item: InboxItem, opts?: { desktopNotify?: boolean }) => void;
  startRealtime: () => void;
  stopRealtime: () => void;
}

const EMPTY_UNREAD: UnreadCounts = { notices: 0, notifications: 0, total: 0 };

let socket: WebSocket | null = null;
let reconnectTimer: number | null = null;
let pollTimer: number | null = null;
let reconnectAttempt = 0;
let started = false;
const desktopNotified = new Set<string>();

function asBool(value: unknown) {
  return value === true || value === 1 || value === "1";
}

function mapNotice(row: ResourceRecord): InboxItem {
  return {
    id: Number(row.id),
    kind: "notice",
    title: String(row.title || ""),
    content: row.content != null ? String(row.content) : undefined,
    noticeType: normalizeNoticeType(row.noticeType),
    priority: normalizeNoticePriority(row.priority),
    read: asBool(row.read),
    at: String(row.publishAt || row.createdAt || ""),
    raw: row,
  };
}

function mapNotification(row: ResourceRecord): InboxItem {
  return {
    id: Number(row.id),
    kind: "notification",
    title: String(row.title || ""),
    summary: String(row.content || ""),
    content: row.content != null ? String(row.content) : undefined,
    channel: String(row.channel || ""),
    read: asBool(row.read),
    at: String(row.createdAt || ""),
    raw: row,
  };
}

function mergeById(items: InboxItem[], next: InboxItem) {
  return [next, ...items.filter((row) => !(row.kind === next.kind && row.id === next.id))];
}

function deriveAlerts(items: InboxItem[]) {
  return {
    marqueeItems: items.filter(
      (item) =>
        item.kind === "notice" &&
        !item.read &&
        normalizeNoticePriority(item.priority) === "urgent",
    ),
  };
}

function notifyDesktopIfNeeded(item: InboxItem) {
  if (item.kind !== "notice" || item.read) return;
  const priority = normalizeNoticePriority(item.priority);
  if (priority !== "important" && priority !== "urgent") return;
  const key = `${item.kind}:${item.id}`;
  if (desktopNotified.has(key)) return;
  desktopNotified.add(key);
  showDesktopNotify({
    title: item.title,
    body: item.content || item.summary || "",
    tag: key,
  });
}

function wsUrl(token: string) {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/ws?token=${encodeURIComponent(token)}`;
}

function clearTimers() {
  if (reconnectTimer != null) {
    window.clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (pollTimer != null) {
    window.clearInterval(pollTimer);
    pollTimer = null;
  }
}

export const useMessagesStore = create<MessagesState>((set, get) => ({
  connected: false,
  items: [],
  unread: EMPTY_UNREAD,
  marqueeItems: [],
  applyUnread: (counts) =>
    set((state) => {
      const next = {
        notices: counts.notices ?? state.unread.notices,
        notifications: counts.notifications ?? state.unread.notifications,
        total: 0,
      };
      next.total =
        counts.total ??
        next.notices + next.notifications;
      return { unread: next };
    }),
  prepend: (item, opts) => {
    if (opts?.desktopNotify) notifyDesktopIfNeeded(item);
    set((state) => {
      const items = mergeById(state.items, item).slice(0, 30);
      return { items, ...deriveAlerts(items) };
    });
  },
  dismissMarquee: (id) =>
    set((state) => ({
      marqueeItems: state.marqueeItems.filter((item) => item.id !== id),
    })),
  removeNotice: (id) => {
    desktopNotified.delete(`notice:${id}`);
    set((state) => {
      const items = state.items.filter(
        (row) => !(row.kind === "notice" && row.id === id),
      );
      return {
        items,
        ...deriveAlerts(items),
      };
    });
  },
  refresh: async () => {
    const [notices, notifications] = await Promise.all([
      listMyNoticesApi({ page: 1, pageSize: 20 }),
      listMyNotificationsApi({ page: 1, pageSize: 10 }),
    ]);
    const noticeItems = (notices.items || []).map(mapNotice);
    const notificationItems = (notifications.items || []).map(mapNotification);
    const merged = [...noticeItems, ...notificationItems].sort((a, b) =>
      String(b.at).localeCompare(String(a.at)),
    );
    set({
      items: merged.slice(0, 20),
      ...deriveAlerts(merged),
    });
    const [noticeUnread, notificationUnread] = await Promise.all([
      listMyNoticesApi({ page: 1, pageSize: 1, unreadOnly: true }),
      listMyNotificationsApi({ page: 1, pageSize: 1, unreadOnly: true }),
    ]);
    const noticesCount = Number(noticeUnread.total || 0);
    const notificationsCount = Number(notificationUnread.total || 0);
    set({
      unread: {
        notices: noticesCount,
        notifications: notificationsCount,
        total: noticesCount + notificationsCount,
      },
    });
  },
  markRead: async (item) => {
    if (item.kind === "notice") await markNoticeReadApi(item.id);
    else await markNotificationReadApi(item.id);
    set((state) => {
      const items = state.items.map((row) =>
        row.kind === item.kind && row.id === item.id ? { ...row, read: true } : row,
      );
      return {
        items,
        ...deriveAlerts(items),
      };
    });
    await get().refresh();
  },
  startRealtime: () => {
    if (started) return;
    started = true;
    void ensureDesktopNotifyPermission();
    const connect = () => {
      const token = useAccessStore.getState().accessToken;
      if (!token) return;
      clearTimers();
      try {
        socket?.close();
      } catch {
        /* ignore */
      }
      socket = new WebSocket(wsUrl(token));
      socket.onopen = () => {
        reconnectAttempt = 0;
        set({ connected: true });
        void get().refresh();
      };
      socket.onclose = () => {
        set({ connected: false });
        if (!started) return;
        const delay = Math.min(30_000, 1000 * 2 ** reconnectAttempt);
        reconnectAttempt += 1;
        reconnectTimer = window.setTimeout(connect, delay);
        if (!pollTimer) {
          pollTimer = window.setInterval(() => {
            void get().refresh();
          }, 15_000);
        }
      };
      socket.onerror = () => {
        socket?.close();
      };
      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(String(event.data)) as {
            type?: string;
            data?: Record<string, unknown>;
          };
          if (message.type === "unread" && message.data) {
            get().applyUnread({
              notices: Number(message.data.notices || 0),
              notifications: Number(message.data.notifications || 0),
              total: Number(message.data.total || 0),
            });
            return;
          }
          if (message.type === "notice_revoked" && message.data) {
            get().removeNotice(Number(message.data.id));
            void get().refresh();
            return;
          }
          if (message.type === "notice" && message.data) {
            get().prepend(
              mapNotice({
                id: Number(message.data.id),
                title: String(message.data.title || ""),
                content: String(message.data.content || ""),
                noticeType: String(message.data.noticeType || ""),
                priority: String(message.data.priority || "normal"),
                publishAt: String(message.data.publishAt || new Date().toISOString()),
                read: false,
              }),
              { desktopNotify: true },
            );
            void get().refresh();
            return;
          }
          if (message.type === "notification" && message.data) {
            get().prepend(mapNotification(message.data as ResourceRecord));
            void get().refresh();
          }
        } catch {
          /* ignore malformed frames */
        }
      };
    };
    connect();
    void get().refresh();
  },
  stopRealtime: () => {
    started = false;
    clearTimers();
    desktopNotified.clear();
    try {
      socket?.close();
    } catch {
      /* ignore */
    }
    socket = null;
    set({
      connected: false,
      items: [],
      unread: EMPTY_UNREAD,
      marqueeItems: [],
    });
  },
}));
