<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, Check, X as XIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useNotificationsStore, type BellItem } from '@/stores/notifications'
import { browserNotifications } from '@/services/browserNotifications'

const props = defineProps<{ collapsed?: boolean }>()

const router = useRouter()
const store = useNotificationsStore()
const isOpen = ref(false)

watch(isOpen, (open) => store.setOpen(open))

const permission = ref<NotificationPermission | 'unsupported'>(browserNotifications.currentPermission())
const showPermissionRow = computed(() =>
  permission.value !== 'granted' && permission.value !== 'unsupported'
)

async function requestPermission() {
  permission.value = await browserNotifications.requestPermission()
}

function relative(ts: number): string {
  const diff = Date.now() - ts
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`
  return `${Math.floor(diff / 86_400_000)}d ago`
}

function onClick(item: BellItem) {
  store.markRead(item.id)
  if (item.link) {
    router.push(item.link)
  }
  isOpen.value = false
}

function dismiss(item: BellItem, e: Event) {
  e.stopPropagation()
  store.remove(item.id)
}
</script>

<template>
  <Popover v-model:open="isOpen">
    <PopoverTrigger as-child>
      <button
        type="button"
        :class="[
          'btn-press relative flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13px] font-medium text-white/50 hover:text-white hover:bg-white/[0.06] light:text-gray-500 light:hover:text-gray-900 light:hover:bg-gray-50 transition-all duration-200',
          props.collapsed && 'md:justify-center md:px-2'
        ]"
        :aria-label="`Notifications${store.unreadCount > 0 ? ` (${store.unreadCount} unread)` : ''}`"
      >
        <Bell class="h-4 w-4 shrink-0" aria-hidden="true" />
        <span :class="props.collapsed && 'md:sr-only'">Notifications</span>
        <Badge
          v-if="store.unreadCount > 0"
          :class="[
            'ml-auto flex-shrink-0 h-5 text-[10px] bg-rose-500/20 text-rose-400 light:bg-rose-100 light:text-rose-700',
            props.collapsed && 'md:absolute md:top-1 md:right-1 md:ml-0 md:h-4 md:min-w-4 md:px-1 md:text-[9px]'
          ]"
        >
          {{ store.unreadCount > 99 ? '99+' : store.unreadCount }}
        </Badge>
      </button>
    </PopoverTrigger>
    <PopoverContent
      side="right"
      align="end"
      class="w-80 p-0 bg-[#0d0d0e] light:bg-white border-white/10 light:border-gray-200"
    >
      <div class="flex items-center justify-between px-3 py-2 border-b border-white/[0.06] light:border-gray-200">
        <span class="text-[12px] font-semibold text-white light:text-gray-900">Notifications</span>
        <button
          v-if="store.items.length > 0"
          type="button"
          class="text-[11px] text-white/50 hover:text-white light:text-gray-500 light:hover:text-gray-900"
          @click="store.clear()"
        >
          Clear all
        </button>
      </div>

      <div
        v-if="showPermissionRow"
        class="flex items-center justify-between gap-2 px-3 py-2 border-b border-white/[0.06] light:border-gray-200 bg-amber-500/10"
      >
        <span class="text-[11px] text-amber-300 light:text-amber-800">
          Enable browser notifications to get alerts even with the tab in the background.
        </span>
        <Button size="sm" class="h-6 px-2 text-[11px]" @click="requestPermission">Enable</Button>
      </div>

      <ScrollArea class="max-h-96">
        <div v-if="store.items.length === 0" class="px-4 py-8 text-center text-[12px] text-white/40 light:text-gray-400">
          You're all caught up.
        </div>
        <ul v-else class="divide-y divide-white/[0.05] light:divide-gray-100">
          <li
            v-for="item in store.items"
            :key="item.id"
            :class="[
              'group flex items-start gap-2 px-3 py-2.5 cursor-pointer transition-colors',
              'hover:bg-white/[0.04] light:hover:bg-gray-50',
              !item.read && 'bg-emerald-500/[0.05]'
            ]"
            @click="onClick(item)"
          >
            <span
              :class="[
                'mt-1.5 h-1.5 w-1.5 rounded-full shrink-0',
                item.read ? 'bg-transparent' : 'bg-emerald-400'
              ]"
              aria-hidden="true"
            />
            <div class="flex-1 min-w-0">
              <div class="text-[12px] font-medium text-white light:text-gray-900 truncate">{{ item.title }}</div>
              <div class="text-[11px] text-white/60 light:text-gray-600 line-clamp-2">{{ item.body }}</div>
              <div class="text-[10px] text-white/40 light:text-gray-400 mt-0.5">{{ relative(item.createdAt) }}</div>
            </div>
            <button
              type="button"
              class="opacity-0 group-hover:opacity-100 text-white/40 hover:text-white light:text-gray-400 light:hover:text-gray-700"
              :aria-label="`Dismiss notification: ${item.title}`"
              @click="dismiss(item, $event)"
            >
              <XIcon class="h-3.5 w-3.5" />
            </button>
          </li>
        </ul>
      </ScrollArea>

      <div
        v-if="store.items.length > 0 && store.unreadCount > 0"
        class="flex items-center justify-end px-3 py-2 border-t border-white/[0.06] light:border-gray-200"
      >
        <button
          type="button"
          class="inline-flex items-center gap-1 text-[11px] text-white/60 hover:text-white light:text-gray-600 light:hover:text-gray-900"
          @click="store.markAllRead()"
        >
          <Check class="h-3 w-3" /> Mark all read
        </button>
      </div>
    </PopoverContent>
  </Popover>
</template>
