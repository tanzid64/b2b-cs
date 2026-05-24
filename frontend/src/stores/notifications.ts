import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Session-only bell notifications. Lives entirely in memory — refresh
// clears the list. Future history/persistence is intentionally out of
// scope; the source-of-truth events are still in WS / the DB.

export type BellKind = 'transfer_pending' | 'agent_new_message'

export interface BellItem {
  id: string             // synthetic — uuidv4-ish from Date.now + random
  kind: BellKind
  title: string
  body: string
  // Route to focus when the user clicks the notification.
  link?: string
  // Domain id (transfer id or contact id) used to dedupe and to dismiss
  // notifications when a related WS event tells us the situation is over
  // (e.g. transfer was picked up by someone else).
  refId?: string
  createdAt: number
  read: boolean
}

function genId() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

const MAX_KEEP = 40

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<BellItem[]>([])
  // Whether the bell popover is currently open. We use this to skip the
  // browser-Notification popup when the user is already looking at the bell.
  const isOpen = ref(false)

  const unreadCount = computed(() => items.value.filter(i => !i.read).length)

  function add(item: Omit<BellItem, 'id' | 'createdAt' | 'read'>) {
    // Dedupe by (kind, refId) — if the same transfer fires twice (e.g.
    // resume + re-create) we update the existing entry instead of stacking.
    if (item.refId) {
      const existing = items.value.find(i => i.kind === item.kind && i.refId === item.refId)
      if (existing) {
        existing.title = item.title
        existing.body = item.body
        existing.link = item.link
        existing.createdAt = Date.now()
        existing.read = false
        // Move to top
        items.value = [existing, ...items.value.filter(i => i.id !== existing.id)]
        return existing
      }
    }
    const bell: BellItem = {
      ...item,
      id: genId(),
      createdAt: Date.now(),
      read: false,
    }
    items.value = [bell, ...items.value].slice(0, MAX_KEEP)
    return bell
  }

  // Remove a notification by its domain refId — used when a WS event tells
  // us the underlying situation is resolved (e.g. transfer picked by peer).
  function dismissByRef(kind: BellKind, refId: string) {
    items.value = items.value.filter(i => !(i.kind === kind && i.refId === refId))
  }

  function markRead(id: string) {
    const it = items.value.find(i => i.id === id)
    if (it) it.read = true
  }

  function markAllRead() {
    items.value.forEach(i => { i.read = true })
  }

  function remove(id: string) {
    items.value = items.value.filter(i => i.id !== id)
  }

  function clear() {
    items.value = []
  }

  function setOpen(open: boolean) {
    isOpen.value = open
    if (open) markAllRead()
  }

  return {
    items,
    isOpen,
    unreadCount,
    add,
    dismissByRef,
    markRead,
    markAllRead,
    remove,
    clear,
    setOpen,
  }
})
