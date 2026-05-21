<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { Mic, Square, Play, Pause, Trash2, Send, Loader2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

const props = withDefaults(defineProps<{
  // Hard cap matches WhatsApp's mobile voice-note ceiling. WhatsApp Cloud
  // API also enforces a 16MB upload limit; 5 min of opus at ~24kbps stays
  // well under that.
  maxDurationSecs?: number
  disabled?: boolean
}>(), {
  maxDurationSecs: 300,
  disabled: false,
})

const emit = defineEmits<{
  (e: 'send', file: File): void
}>()

type State = 'idle' | 'recording' | 'preview' | 'sending'
const state = ref<State>('idle')

const mediaRecorder = ref<MediaRecorder | null>(null)
const mediaStream = ref<MediaStream | null>(null)
const chunks = ref<Blob[]>([])
const recordedBlob = ref<Blob | null>(null)
const recordedMime = ref<string>('')
const elapsedSecs = ref(0)
const elapsedTimer = ref<number | null>(null)
const audioEl = ref<HTMLAudioElement | null>(null)
const previewUrl = ref<string | null>(null)
const isPlaying = ref(false)

// Pick the best MIME the browser supports. WhatsApp prefers ogg/opus for
// voice notes; webm/opus is the safer Chromium default; mp4 is Safari's
// only option. Order matters — first supported wins.
function pickMimeType(): string {
  const candidates = [
    'audio/ogg;codecs=opus',
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/mp4',
  ]
  for (const m of candidates) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(m)) {
      return m
    }
  }
  return ''
}

function formatTime(s: number): string {
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${m}:${r.toString().padStart(2, '0')}`
}

const timeLabel = computed(() => formatTime(elapsedSecs.value))
const maxLabel = computed(() => formatTime(props.maxDurationSecs))

async function startRecording() {
  if (props.disabled || state.value !== 'idle') return

  const mime = pickMimeType()
  if (!mime) {
    toast.error('Voice recording is not supported in this browser')
    return
  }

  try {
    mediaStream.value = await navigator.mediaDevices.getUserMedia({ audio: true })
  } catch (err: any) {
    if (err?.name === 'NotAllowedError') {
      toast.error('Microphone permission denied', {
        description: 'Allow microphone access in your browser settings to record voice notes.',
      })
    } else {
      toast.error('Could not access microphone', { description: err?.message })
    }
    return
  }

  chunks.value = []
  recordedMime.value = mime

  const rec = new MediaRecorder(mediaStream.value, { mimeType: mime })
  mediaRecorder.value = rec

  rec.ondataavailable = e => {
    if (e.data && e.data.size > 0) chunks.value.push(e.data)
  }
  rec.onstop = () => {
    recordedBlob.value = new Blob(chunks.value, { type: mime })
    previewUrl.value = URL.createObjectURL(recordedBlob.value)
    state.value = 'preview'
    stopStream()
  }

  rec.start()
  state.value = 'recording'
  elapsedSecs.value = 0
  elapsedTimer.value = window.setInterval(() => {
    elapsedSecs.value += 1
    if (elapsedSecs.value >= props.maxDurationSecs) stopRecording()
  }, 1000)
}

function stopRecording() {
  if (state.value !== 'recording') return
  if (elapsedTimer.value !== null) {
    clearInterval(elapsedTimer.value)
    elapsedTimer.value = null
  }
  mediaRecorder.value?.stop()
}

function stopStream() {
  mediaStream.value?.getTracks().forEach(t => t.stop())
  mediaStream.value = null
  mediaRecorder.value = null
}

function togglePlay() {
  if (!audioEl.value) return
  if (isPlaying.value) audioEl.value.pause()
  else audioEl.value.play()
}

function cancel() {
  if (state.value === 'recording') {
    if (elapsedTimer.value !== null) {
      clearInterval(elapsedTimer.value)
      elapsedTimer.value = null
    }
    try { mediaRecorder.value?.stop() } catch { /* already stopped */ }
    stopStream()
  }
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
  }
  recordedBlob.value = null
  chunks.value = []
  elapsedSecs.value = 0
  isPlaying.value = false
  state.value = 'idle'
}

function send() {
  if (!recordedBlob.value) return
  // ChatView's uploader infers the file kind from mime, so the filename
  // extension is only cosmetic. Picking .ogg / .webm / .m4a keeps the
  // server log readable.
  const ext = recordedMime.value.includes('ogg') ? 'ogg'
    : recordedMime.value.includes('mp4') ? 'm4a'
    : 'webm'
  const file = new File(
    [recordedBlob.value],
    `voice-note-${Date.now()}.${ext}`,
    { type: recordedMime.value },
  )
  state.value = 'sending'
  emit('send', file)
}

// Parent calls this after a successful upload to reset the recorder.
function reset() {
  cancel()
}
defineExpose({ reset })

onBeforeUnmount(() => {
  if (elapsedTimer.value !== null) clearInterval(elapsedTimer.value)
  stopStream()
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})
</script>

<template>
  <!-- Idle: mic button matches the Paperclip's visual style. -->
  <Tooltip v-if="state === 'idle'">
    <TooltipTrigger as-child>
      <button
        type="button"
        class="w-9 h-9 rounded-lg hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center transition-colors disabled:opacity-50"
        :disabled="disabled"
        @click="startRecording"
      >
        <Mic class="w-[18px] h-[18px] text-white/40 light:text-gray-500" />
      </button>
    </TooltipTrigger>
    <TooltipContent>Record voice note</TooltipContent>
  </Tooltip>

  <!-- Recording: timer + stop. Pulsing red dot signals "live". -->
  <div
    v-else-if="state === 'recording'"
    class="flex items-center gap-2 flex-1 bg-red-500/10 light:bg-red-50 border border-red-500/30 light:border-red-200 rounded-lg px-3 py-1.5"
  >
    <span class="relative flex h-2.5 w-2.5">
      <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-500 opacity-75" />
      <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-red-500" />
    </span>
    <span class="text-[13px] font-mono text-white light:text-gray-900 tabular-nums">
      {{ timeLabel }} <span class="text-white/40 light:text-gray-400">/ {{ maxLabel }}</span>
    </span>
    <span class="text-[12px] text-white/50 light:text-gray-500">Recording…</span>
    <div class="ml-auto flex items-center gap-1">
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="w-8 h-8 rounded-lg hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center"
            @click="cancel"
          >
            <Trash2 class="w-4 h-4 text-white/60 light:text-gray-600" />
          </button>
        </TooltipTrigger>
        <TooltipContent>Discard</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="w-8 h-8 rounded-lg bg-red-500 hover:bg-red-400 flex items-center justify-center"
            @click="stopRecording"
          >
            <Square class="w-3.5 h-3.5 text-white fill-white" />
          </button>
        </TooltipTrigger>
        <TooltipContent>Stop</TooltipContent>
      </Tooltip>
    </div>
  </div>

  <!-- Preview: play/pause + delete + send. -->
  <div
    v-else-if="state === 'preview' || state === 'sending'"
    class="flex items-center gap-2 flex-1 bg-white/[0.04] light:bg-gray-100 border border-white/[0.08] light:border-gray-200 rounded-lg px-3 py-1.5"
  >
    <button
      type="button"
      class="w-8 h-8 rounded-full bg-emerald-600 hover:bg-emerald-500 flex items-center justify-center disabled:opacity-50"
      :disabled="state === 'sending'"
      @click="togglePlay"
    >
      <Pause v-if="isPlaying" class="w-4 h-4 text-white" />
      <Play v-else class="w-4 h-4 text-white ml-0.5" />
    </button>
    <span class="text-[13px] font-mono text-white/70 light:text-gray-700 tabular-nums">
      {{ timeLabel }}
    </span>
    <audio
      v-if="previewUrl"
      ref="audioEl"
      :src="previewUrl"
      class="hidden"
      @play="isPlaying = true"
      @pause="isPlaying = false"
      @ended="isPlaying = false"
    />
    <div class="ml-auto flex items-center gap-1">
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="w-8 h-8 rounded-lg hover:bg-white/[0.08] light:hover:bg-gray-200 flex items-center justify-center disabled:opacity-50"
            :disabled="state === 'sending'"
            @click="cancel"
          >
            <Trash2 class="w-4 h-4 text-white/60 light:text-gray-600" />
          </button>
        </TooltipTrigger>
        <TooltipContent>Discard</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger as-child>
          <button
            type="button"
            class="w-8 h-8 rounded-lg bg-emerald-600 hover:bg-emerald-500 flex items-center justify-center disabled:opacity-50"
            :disabled="state === 'sending'"
            @click="send"
          >
            <Loader2 v-if="state === 'sending'" class="w-4 h-4 text-white animate-spin" />
            <Send v-else class="w-4 h-4 text-white" />
          </button>
        </TooltipTrigger>
        <TooltipContent>Send</TooltipContent>
      </Tooltip>
    </div>
  </div>
</template>
