/** Show a Windows / OS notification when permission allows. */
export async function ensureDesktopNotifyPermission() {
  if (typeof window === 'undefined' || !('Notification' in window)) return 'denied';
  if (Notification.permission === 'granted' || Notification.permission === 'denied') {
    return Notification.permission;
  }
  try {
    return await Notification.requestPermission();
  } catch {
    return 'denied';
  }
}

export function showDesktopNotify(input: {
  title: string;
  body?: string;
  tag?: string;
  onClick?: () => void;
}) {
  if (typeof window === 'undefined' || !('Notification' in window)) return;
  if (Notification.permission !== 'granted') return;
  try {
    const options: NotificationOptions & { renotify?: boolean } = {
      body: input.body || '',
      tag: input.tag,
      renotify: true,
    };
    const note = new Notification(input.title, options);
    if (input.onClick) {
      note.onclick = () => {
        window.focus();
        input.onClick?.();
        note.close();
      };
    }
  } catch {
    /* ignore unsupported environments */
  }
}
