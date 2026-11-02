import { ref } from 'vue'

const current = ref('')
let hideTimer: ReturnType<typeof setTimeout> | null = null

const VISIBLE_MS = 2000

export function useToast() {
  function show(text: string) {
    const next = text.trim()
    if (!next) return
    current.value = next
    if (hideTimer) clearTimeout(hideTimer)
    hideTimer = setTimeout(() => {
      current.value = ''
      hideTimer = null
    }, VISIBLE_MS)
  }

  return { current, show }
}
