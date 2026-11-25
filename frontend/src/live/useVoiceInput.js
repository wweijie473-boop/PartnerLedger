import { computed, getCurrentInstance, onUnmounted, ref } from 'vue'

const MAX_RECORD_SECONDS = 55
const TARGET_SAMPLE_RATE = 16000

export function encodeMonoWavBase64(chunks, inputSampleRate, targetRate = TARGET_SAMPLE_RATE) {
  const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0)
  const merged = new Float32Array(totalLength)
  let offset = 0
  for (const chunk of chunks) {
    merged.set(chunk, offset)
    offset += chunk.length
  }

  let samples = merged
  if (inputSampleRate > targetRate && merged.length > 0) {
    const ratio = inputSampleRate / targetRate
    const outLength = Math.max(1, Math.round(merged.length / ratio))
    samples = new Float32Array(outLength)
    for (let i = 0; i < outLength; i++) {
      const start = Math.floor(i * ratio)
      const end = Math.min(merged.length, Math.floor((i + 1) * ratio))
      let sum = 0
      let count = 0
      for (let j = start; j < end; j++) {
        sum += merged[j]
        count++
      }
      samples[i] = count > 0 ? sum / count : merged[Math.min(start, merged.length - 1)]
    }
  } else {
    targetRate = inputSampleRate
  }

  const dataByteLength = samples.length * 2
  const buffer = new ArrayBuffer(44 + dataByteLength)
  const view = new DataView(buffer)
  const writeAscii = (pos, str) => {
    for (let i = 0; i < str.length; i++) view.setUint8(pos + i, str.charCodeAt(i))
  }

  writeAscii(0, 'RIFF')
  view.setUint32(4, 36 + dataByteLength, true)
  writeAscii(8, 'WAVE')
  writeAscii(12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true)
  view.setUint16(22, 1, true)
  view.setUint32(24, targetRate, true)
  view.setUint32(28, targetRate * 2, true)
  view.setUint16(32, 2, true)
  view.setUint16(34, 16, true)
  writeAscii(36, 'data')
  view.setUint32(40, dataByteLength, true)

  let byteOffset = 44
  for (let i = 0; i < samples.length; i++) {
    const clamped = Math.max(-1, Math.min(1, samples[i]))
    view.setInt16(byteOffset, clamped < 0 ? clamped * 0x8000 : clamped * 0x7fff, true)
    byteOffset += 2
  }

  const bytes = new Uint8Array(buffer)
  if (typeof Buffer !== 'undefined') {
    return Buffer.from(bytes).toString('base64')
  }
  let binary = ''
  const chunkSize = 0x8000
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize))
  }
  return btoa(binary)
}

export function appendTranscript(existing, addition) {
  const cleanAdd = String(addition ?? '').trim()
  if (!cleanAdd) return existing
  const base = String(existing ?? '')
  if (!base.trim()) return cleanAdd
  const needsSpace = /[a-z0-9]$/i.test(base.trimEnd()) && /^[a-z0-9]/i.test(cleanAdd)
  return base.trimEnd() + (needsSpace ? ' ' : '') + cleanAdd
}

export function useVoiceInput({ demo, request, input, onReady }) {
  const recording = ref(false)
  const transcribing = ref(false)
  const seconds = ref(0)
  const voiceError = ref('')

  const formattedDuration = computed(() => {
    const mins = String(Math.floor(seconds.value / 60)).padStart(2, '0')
    const secs = String(seconds.value % 60).padStart(2, '0')
    return `${mins}:${secs}`
  })

  let activeCleanup = null
  let timer = null

  function clearTimer() {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  function startTimer(onMaxReached) {
    seconds.value = 0
    clearTimer()
    timer = setInterval(() => {
      seconds.value += 1
      if (seconds.value >= MAX_RECORD_SECONDS) {
        onMaxReached()
      }
    }, 1000)
  }

  async function startBackendAudioRecording() {
    if (!globalThis.navigator?.mediaDevices?.getUserMedia) {
      voiceError.value = '当前浏览器不支持麦克风录音，请使用最新版 Chrome 或 Edge。'
      return
    }
    let stream
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true },
      })
    } catch (error) {
      voiceError.value = error?.name === 'NotAllowedError' || error?.name === 'PermissionDeniedError'
        ? '未获得麦克风权限，请在浏览器地址栏允许访问麦克风。'
        : '未检测到可用麦克风设备，请检查系统音频输入。'
      return
    }

    const AudioCtx = globalThis.AudioContext || globalThis.webkitAudioContext
    const audioCtx = new AudioCtx()
    if (audioCtx.state === 'suspended') {
      await audioCtx.resume().catch(() => {})
    }
    const source = audioCtx.createMediaStreamSource(stream)
    const processor = audioCtx.createScriptProcessor(4096, 1, 1)
    const muteGain = audioCtx.createGain()
    muteGain.gain.value = 0
    const chunks = []
    const sampleRate = audioCtx.sampleRate || 48000

    processor.onaudioprocess = event => {
      if (!recording.value) return
      const channel = event.inputBuffer.getChannelData(0)
      chunks.push(new Float32Array(channel))
    }

    source.connect(processor)
    processor.connect(muteGain)
    muteGain.connect(audioCtx.destination)
    recording.value = true

    const finish = async (cancel = false) => {
      if (!recording.value && !cancel) return
      recording.value = false
      clearTimer()
      activeCleanup = null
      try { processor.disconnect(); source.disconnect(); muteGain.disconnect() } catch {}
      for (const track of stream.getTracks()) track.stop()
      await audioCtx.close().catch(() => {})
      if (cancel) return

      const sampleCount = chunks.reduce((sum, item) => sum + item.length, 0)
      if (sampleCount < sampleRate * 0.25) {
        voiceError.value = '说话时间太短，请点击语音按钮后说出完整业务指令。'
        return
      }

      transcribing.value = true
      try {
        const audio = encodeMonoWavBase64(chunks, sampleRate, TARGET_SAMPLE_RATE)
        const res = await request('/api/ai/transcribe', {
          method: 'POST',
          body: { audio, format: 'wav' },
          timeout: 30000,
        })
        const text = String(res?.text ?? '').trim()
        if (!text) {
          voiceError.value = '未识别到清晰语音，请靠近麦克风重试。'
          return
        }
        input.value = appendTranscript(input.value, text)
        onReady?.()
      } catch (cause) {
        voiceError.value = cause?.message || '语音转写失败，请稍后重试或直接输入文字。'
      } finally {
        transcribing.value = false
      }
    }

    activeCleanup = finish
    startTimer(() => finish(false))
  }

  function startBrowserSpeechRecognition() {
    const SpeechRec = globalThis.SpeechRecognition || globalThis.webkitSpeechRecognition
    if (!SpeechRec) {
      voiceError.value = '本地演示模式依赖浏览器原生语音识别，请使用 Edge/Chrome 或登录正式工作台使用云端语音转写。'
      return
    }
    const recognition = new SpeechRec()
    recognition.lang = 'zh-CN'
    recognition.continuous = true
    recognition.interimResults = true
    const initialBase = input.value
    let confirmed = ''

    recognition.onresult = event => {
      let interim = ''
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const transcript = event.results[i][0]?.transcript || ''
        if (event.results[i].isFinal) confirmed += transcript
        else interim += transcript
      }
      input.value = appendTranscript(initialBase, confirmed + interim)
    }

    recognition.onerror = event => {
      const code = event?.error
      if (code === 'aborted') return
      const messages = {
        'not-allowed': '未获得麦克风权限，请在浏览器地址栏允许访问麦克风。',
        'service-not-allowed': '当前浏览器禁止原生语音服务，请登录正式工作台使用大模型语音转写。',
        'audio-capture': '未检测到可用麦克风设备。',
        'no-speech': '未检测到清晰语音，请重试。',
        network: '浏览器内置语音服务连接受限，请登录正式工作台使用大模型语音转写。',
      }
      voiceError.value = messages[code] || '语音识别中断，请重试或直接输入文字。'
      recording.value = false
      clearTimer()
      activeCleanup = null
    }

    recognition.onend = () => {
      if (recording.value) {
        recording.value = false
        clearTimer()
        activeCleanup = null
        onReady?.()
      }
    }

    try {
      recognition.start()
      recording.value = true
      const finish = (cancel = false) => {
        recording.value = false
        clearTimer()
        activeCleanup = null
        try {
          if (cancel) recognition.abort()
          else recognition.stop()
        } catch {}
        if (!cancel) onReady?.()
      }
      activeCleanup = finish
      startTimer(() => finish(false))
    } catch {
      voiceError.value = '无法启动语音识别，请重试。'
    }
  }

  async function toggleVoice() {
    if (transcribing.value) return
    voiceError.value = ''
    if (recording.value && activeCleanup) {
      await activeCleanup(false)
      return
    }
    if (demo) {
      startBrowserSpeechRecognition()
    } else {
      await startBackendAudioRecording()
    }
  }

  function cancelVoice() {
    if (activeCleanup) activeCleanup(true)
    clearTimer()
    recording.value = false
  }

  if (getCurrentInstance()) {
    onUnmounted(cancelVoice)
  }

  return {
    recording,
    transcribing,
    seconds,
    formattedDuration,
    voiceError,
    toggleVoice,
    cancelVoice,
  }
}
