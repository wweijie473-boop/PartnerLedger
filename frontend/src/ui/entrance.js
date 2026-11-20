const queued = new WeakSet()
const active = new WeakMap()
const groups = new WeakMap()
let viewport

function observe(element, enter) {
  if (!('IntersectionObserver' in window)) {
    enter()
    return
  }
  viewport ??= new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (entry.isIntersecting) active.get(entry.target)?.enter()
    }
  })
  viewport.observe(element)
}

function enqueue(element, position = 0) {
  if (queued.has(element)) return
  queued.add(element)
  const preference = window.matchMedia('(prefers-reduced-motion: no-preference)')
  if (!preference.matches || !element.animate || element.matches(':focus-within')) return

  const index = Number.isFinite(position) ? Math.max(0, Math.min(position, 4)) : 0
  let animation

  function finish() {
    viewport?.unobserve(element)
    element.removeAttribute('data-ui-entrance-pending')
    element.removeEventListener('focusin', cancel)
    preference.removeEventListener('change', cancel)
    active.delete(element)
  }

  function cancel() {
    animation?.cancel()
    finish()
  }

  function enter() {
    if (!element.hasAttribute('data-ui-entrance-pending')) return
    viewport?.unobserve(element)
    element.removeAttribute('data-ui-entrance-pending')
    if (!preference.matches || element.matches(':focus-within')) {
      finish()
      return
    }
    const style = getComputedStyle(element)
    const duration = style.getPropertyValue('--ui-motion-emphasized').trim()
    animation = element.animate(
      [{ opacity: 0.35, translate: '0 14px' }, { opacity: 1, translate: '0 0' }],
      {
        delay: index * 55,
        duration: Number.parseFloat(duration) * (duration.endsWith('ms') ? 1 : 1000) || 480,
        easing: style.getPropertyValue('--ui-ease-out').trim() || 'ease-out',
        fill: 'backwards',
      },
    )
    animation.finished.then(finish, finish)
  }

  active.set(element, { enter, cancel })
  element.setAttribute('data-ui-entrance-pending', '')
  element.addEventListener('focusin', cancel)
  preference.addEventListener('change', cancel)
  observe(element, enter)
}

function updateGroup(element, start) {
  const children = new Set(element.children)
  for (const child of groups.get(element) ?? []) {
    if (!children.has(child)) active.get(child)?.cancel()
  }
  groups.set(element, children)
  let position = Number.isFinite(start) ? start : 0
  for (const child of children) enqueue(child, position++)
}

export const entrance = {
  mounted(element, { arg, value }) {
    if (arg === 'children') updateGroup(element, value)
    else enqueue(element, value)
  },
  updated(element, { arg, value }) {
    if (arg === 'children') updateGroup(element, value)
  },
  beforeUnmount(element) {
    active.get(element)?.cancel()
    for (const child of groups.get(element) ?? []) active.get(child)?.cancel()
    groups.delete(element)
  },
}
