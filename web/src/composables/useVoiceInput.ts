import { ref, readonly } from 'vue'
import { transcribeAudio } from '@/services/speechToText'

const MAX_RECORDING_MS = 60_000 // 60 seconds

export function useVoiceInput() {
  const recording = ref(false)
  let mediaRecorder: MediaRecorder | null = null
  const audioChunks: Blob[] = []
  let stream: MediaStream | null = null
  let timeoutId: ReturnType<typeof setTimeout> | null = null

  async function startRecording(): Promise<void> {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : 'audio/webm'
    const recorder = new MediaRecorder(stream, { mimeType })
    audioChunks.length = 0

    recorder.ondataavailable = (e: BlobEvent) => {
      if (e.data.size > 0) audioChunks.push(e.data)
    }

    recorder.start()
    mediaRecorder = recorder
    recording.value = true

    // Auto-stop after max duration.
    timeoutId = setTimeout(() => {
      if (recording.value) {
        stopRecording().catch(() => {})
      }
    }, MAX_RECORDING_MS)
  }

  async function stopRecording(): Promise<string> {
    return new Promise((resolve, reject) => {
      const recorder = mediaRecorder
      if (!recorder) {
        recording.value = false
        return reject(new Error('not recording'))
      }

      if (timeoutId) {
        clearTimeout(timeoutId)
        timeoutId = null
      }

      recorder.onstop = async () => {
        // Stop all tracks.
        stream?.getTracks().forEach((t) => t.stop())
        stream = null
        mediaRecorder = null
        recording.value = false

        try {
          const blob = new Blob(audioChunks, { type: recorder.mimeType })
          const text = await transcribeAudio(blob)
          resolve(text)
        } catch (e) {
          reject(e)
        }
      }

      recorder.stop()
    })
  }

  function cancelRecording(): void {
    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }
    const recorder = mediaRecorder
    if (recorder) {
      // Prevent onstop from trying to transcribe.
      recorder.onstop = null
      recorder.stop()
    }
    stream?.getTracks().forEach((t) => t.stop())
    stream = null
    mediaRecorder = null
    audioChunks.length = 0
    recording.value = false
  }

  return {
    recording: readonly(recording),
    startRecording,
    stopRecording,
    cancelRecording,
  }
}
