const running = new WeakMap()

function fade(element) {
  running.get(element)?.cancel()
  if (!window.matchMedia('(prefers-reduced-motion: no-preference)').matches || !element.animate) return

  const theme = getComputedStyle(element)
  const duration = Number.parseFloat(theme.getPropertyValue('--ui-motion-fast'))
  const opacity = Number.parseFloat(theme.getPropertyValue('--ui-page-entry-opacity'))
  const easing = theme.getPropertyValue('--ui-ease-out').trim()
  running.set(element, element.animate([{ opacity }, { opacity: 1 }], { duration, easing }))
}

export const vPageFade = {
  mounted(element, { modifiers }) {
    if (!modifiers.changes) fade(element)
  },
  updated(element, { value, oldValue }) {
    if (value !== oldValue) fade(element)
  },
  beforeUnmount(element) {
    running.get(element)?.cancel()
    running.delete(element)
  },
}
