<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { UiIcon } from './ui/index.js'

const width = 430
const height = 250
const duration = 14000
const angles = Array.from({ length: 128 }, (_, index) => index * 2 * Math.PI / 128)
const surface = ref(null)
const ready = ref(false)
const paused = ref(false)
const reducedMotion = ref(false)
let frame
let motionPreference
let viewportObserver
let paint
let elapsed = 0
let inView = false

function radialWash(context, x, y, radius, color) {
  const wash = context.createRadialGradient(x, y, 0, x, y, radius)
  wash.addColorStop(0, color)
  wash.addColorStop(1, 'transparent')
  return wash
}

function inkGradient(context, colors) {
  const ink = context.createLinearGradient(40, 168, 395, 69)
  for (const [index, color] of colors.entries()) ink.addColorStop(index / (colors.length - 1), color)
  return ink
}

function painter(context) {
  const blueWash = radialWash(context, 173, 134, 130, '#317ed640')
  const violetWash = radialWash(context, 312, 100, 97, '#a889ee30')
  const inks = [
    inkGradient(context, ['#3278d2', '#54b7ff', '#8b9dfc', '#d4abf3']),
    inkGradient(context, ['#46a6f5', '#78ddf1', '#99a8ff', '#eec6ff']),
    inkGradient(context, ['#3a75d1', '#59b3f2', '#b2b1ff', '#dcb4ee']),
  ]

  return (progress) => {
    const phase = progress * 2 * Math.PI
    const bend = 0.42 * Math.sin(phase)
    const tilt = -0.15 + 0.14 * Math.sin(phase - 0.45)
    const cosTilt = Math.cos(tilt)
    const sinTilt = Math.sin(tilt)
    // Each strand follows the same closed curve at a different offset across the fold.
    const fold = angles.map(angle => {
      const x = 127 * Math.cos(angle) + 27 * Math.cos(2 * angle - 0.38 + bend)
      const y = 47 * Math.sin(2 * angle + 0.34 + bend) + 24 * Math.sin(angle + 0.46) - 10 * Math.cos(angle)
      const dx = 33 * Math.cos(angle + 0.85 + bend) + 7 * Math.cos(3 * angle)
      const dy = 31 * Math.sin(angle + 0.52 + bend) + 13 * Math.cos(2 * angle - 0.5)
      return {
        x: 207 + x * cosTilt - y * sinTilt,
        y: 124 + x * sinTilt + y * cosTilt,
        dx: dx * cosTilt - dy * sinTilt,
        dy: dx * sinTilt + dy * cosTilt,
      }
    })

    context.clearRect(0, 0, width, height)
    context.globalAlpha = 1
    context.fillStyle = blueWash
    context.fillRect(0, 0, width, height)
    context.fillStyle = violetWash
    context.fillRect(0, 0, width, height)

    const strands = 54
    for (let strand = 0; strand < strands; strand++) {
      const fraction = strand / (strands - 1)
      const offset = 2 * fraction - 1
      context.beginPath()
      const first = fold[0]
      context.moveTo(first.x + offset * first.dx, first.y + offset * first.dy)
      for (let index = 1; index < fold.length; index++) {
        const { x, y, dx, dy } = fold[index]
        context.lineTo(x + offset * dx, y + offset * dy)
      }
      context.closePath()
      context.strokeStyle = inks[Math.min(inks.length - 1, Math.floor(fraction * inks.length))]
      context.lineWidth = strand % 9 === 0 ? 1 : 0.64
      context.globalAlpha = 0.42 + 0.32 * Math.abs(offset)
      context.stroke()
    }
  }
}

function updatePlayback() {
  window.cancelAnimationFrame(frame)
  if (paused.value || reducedMotion.value || !inView || document.hidden) return

  const offset = elapsed
  let start
  let lastPaint = -Infinity
  function animate(now) {
    start ??= now
    if (now - lastPaint >= 1000 / 30) {
      elapsed = (offset + now - start) % duration
      paint(elapsed / duration)
      lastPaint = now
    }
    frame = window.requestAnimationFrame(animate)
  }
  frame = window.requestAnimationFrame(animate)
}

function toggleMotion() {
  paused.value = !paused.value
  updatePlayback()
}

function updatePreference() {
  reducedMotion.value = motionPreference.matches
  updatePlayback()
}

onMounted(() => {
  const context = surface.value.getContext('2d')
  if (!context) return
  context.scale(2, 2)
  paint = painter(context)
  paint(0)
  motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)')
  motionPreference.addEventListener('change', updatePreference)
  document.addEventListener('visibilitychange', updatePlayback)
  if ('IntersectionObserver' in window) {
    viewportObserver = new IntersectionObserver(([entry]) => {
      inView = entry.isIntersecting
      updatePlayback()
    })
    viewportObserver.observe(surface.value)
  } else {
    inView = true
  }
  updatePreference()
  ready.value = true
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(frame)
  viewportObserver?.disconnect()
  motionPreference?.removeEventListener('change', updatePreference)
  document.removeEventListener('visibilitychange', updatePlayback)
})
</script>

<template>
  <canvas ref="surface" class="pl-visual" :width="width * 2" :height="height * 2" aria-hidden="true"></canvas>
  <button v-if="ready && !reducedMotion" class="pl-motion-toggle" type="button"
    :aria-label="paused ? '播放背景动效' : '暂停背景动效'" :title="paused ? '播放背景动效' : '暂停背景动效'"
    @click="toggleMotion">
    <UiIcon :name="paused ? 'play' : 'pause'" :size="16" />
  </button>
</template>
