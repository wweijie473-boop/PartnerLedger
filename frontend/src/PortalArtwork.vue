<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'

const width = 430
const height = 250
const duration = 3800
const angles = Array.from({ length: 128 }, (_, index) => index * 2 * Math.PI / 128)
const surface = ref(null)
let frame
let motionPreference
let play

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

onMounted(() => {
  const context = surface.value.getContext('2d')
  if (!context) return
  context.scale(2, 2)
  const paint = painter(context)
  motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)')
  play = () => {
    window.cancelAnimationFrame(frame)
    paint(0)
    if (motionPreference.matches) return
    let start
    const animate = (now) => {
      start ??= now
      const progress = Math.min((now - start) / duration, 1)
      paint(progress)
      if (progress < 1) frame = window.requestAnimationFrame(animate)
    }
    frame = window.requestAnimationFrame(animate)
  }
  motionPreference.addEventListener('change', play)
  play()
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(frame)
  motionPreference?.removeEventListener('change', play)
})
</script>

<template>
  <canvas ref="surface" class="pl-visual" :width="width * 2" :height="height * 2" aria-hidden="true"></canvas>
</template>
