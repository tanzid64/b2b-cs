// Thin wrapper around the foreground Notification API. Push-when-tab-closed
// (service worker + VAPID) will be layered on top of this in a follow-up.

export interface BrowserNotificationOptions {
  title: string
  body: string
  /** Path to focus when the notification is clicked. */
  link?: string
  /** Dedup id — newer notification with the same tag replaces the older one. */
  tag?: string
}

function isSupported(): boolean {
  return typeof window !== 'undefined' && 'Notification' in window
}

function currentPermission(): NotificationPermission | 'unsupported' {
  if (!isSupported()) return 'unsupported'
  return Notification.permission
}

async function requestPermission(): Promise<NotificationPermission | 'unsupported'> {
  if (!isSupported()) return 'unsupported'
  if (Notification.permission === 'granted' || Notification.permission === 'denied') {
    return Notification.permission
  }
  try {
    return await Notification.requestPermission()
  } catch {
    return Notification.permission
  }
}

// Show a foreground notification. Fires whenever permission is granted —
// even when the tab is focused — so agents always get the OS popup. The
// in-app bell still renders independently; the two coexist.
function show(opts: BrowserNotificationOptions): Notification | null {
  if (!isSupported()) return null
  if (Notification.permission !== 'granted') return null
  try {
    const n = new Notification(opts.title, {
      body: opts.body,
      tag: opts.tag,
      icon: '/favicon.ico',
      // renotify is best-effort and not supported everywhere — safe to set.
      ...(opts.tag ? { renotify: true } : {}),
    } as NotificationOptions)
    if (opts.link) {
      n.onclick = () => {
        window.focus()
        // Use SPA navigation if possible by setting hash/path — defer to
        // the bell-click handler in the app for the actual route push.
        window.location.assign(opts.link!)
        n.close()
      }
    }
    return n
  } catch {
    return null
  }
}

export const browserNotifications = {
  isSupported,
  currentPermission,
  requestPermission,
  show,
}
