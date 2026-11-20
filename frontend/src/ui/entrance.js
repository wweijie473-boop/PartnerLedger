const queued = new WeakSet()
const active = new WeakMap()
const groups = new WeakMap()
let viewport

function observe(element, enter) {
  if (!('IntersectionObserver' in window)) {
    enter()
    return
  }
  if (!viewport) {
    const desktop = window.matchMedia('(min-width: 1084px)')
    viewport = new IntersectionObserver(entries => {
      const threshold = desktop.matches ? 0.3 : 0.1
      for (const entry of entries) {
        if (!entry.isIntersecting) continue
        // A tall form or table can exceed the viewport several times over.
        const tallerThanViewport = entry.rootBounds && entry.boundingClientRect.height > entry.rootBounds.height
        if (entry.intersectionRatio >= threshold || tallerThanViewport) active.get(entry.target)?.enter()
      }
    }, { threshold: [0, 0.1, 0.3] })
    desktop.addEventListener('change', () => {
      for (const pending of document.querySelectorAll('[data-ui-entrance-pending]')) {
        viewport.unobserve(pending)
        viewport.observe(pending)
      }
    })
  }
  viewport.observe(element)
}

// Read the laid-out row at playback time so wrapped and mobile cards start a new sequence.
function columnInRow(element) {
  const top = element.offsetTop
  let previous = element.previousElementSibling
  if (previous?.offsetTop !== top && element.nextElementSibling?.offsetTop !== top) return null
  let column = 0
  while (previous?.offsetTop === top && column < 5) {
    column++
    previous = previous.previousElementSibling
  }
  return column
}

function enqueue(element, position = 0, kind = 'module', after) {
  if (queued.has(element)) return
  queued.add(element)
  const preference = window.matchMedia('(prefers-reduced-motion: no-preference)')
  if (!preference.matches || !element.animate || element.matches(':focus-within')) return

  let animation
  let finished

  function finish() {
    viewport?.unobserve(element)
    element.removeAttribute('data-ui-entrance-pending')
    element.removeAttribute('data-ui-entrance-card')
    element.removeEventListener('focusin', cancel)
    preference.removeEventListener('change', cancel)
    active.delete(element)
  }

  function cancel() {
    animation?.cancel()
    finish()
  }

  function enter() {
    if (!element.hasAttribute('data-ui-entrance-pending')) return finished
    viewport?.unobserve(element)
    element.removeAttribute('data-ui-entrance-pending')
    if (!preference.matches || element.matches(':focus-within')) {
      finish()
      return
    }
    const isRow = kind === 'row'
    if (isRow && (element.childElementCount < 2 || element.firstElementChild.offsetTop !== element.lastElementChild.offsetTop)) {
      finish()
      return
    }
    const column = kind === 'child' ? columnInRow(element) : null
    const index = Number.isFinite(position) ? Math.max(0, Math.min(position, 5)) : 0
    const distance = column === null ? 100 : 75 + 150 * column
    if (column !== null) element.setAttribute('data-ui-entrance-card', '')
    const frames = isRow
      ? [{ opacity: 0 }, { opacity: 1 }]
      : [{ opacity: 0, translate: `0 ${distance}px` }, { opacity: 1, translate: '0 0' }]
    const delay = isRow ? 50 : (column === null ? index : column + 1) * 25
    animation = element.animate(frames, { delay, duration: 1200, easing: 'ease-in-out', fill: 'backwards' })
    finished = animation.finished.then(finish, finish)
    return finished
  }

  function enterInOrder() {
    const previous = after()
    const previousEntry = active.get(previous)
    if (previousEntry && !element.matches(':target')) {
      const bounds = previous.getBoundingClientRect()
      if (bounds.bottom > 0 && bounds.top < window.innerHeight) {
        const done = previousEntry.enter()
        if (done) {
          viewport?.unobserve(element)
          done.then(() => {
            if (active.has(element)) observe(element, enter)
          })
          return
        }
      }
    }
    enter()
  }

  const onVisible = after ? enterInOrder : enter
  active.set(element, { enter: onVisible, cancel })
  element.setAttribute('data-ui-entrance-pending', kind)
  element.addEventListener('focusin', cancel)
  preference.addEventListener('change', cancel)
  observe(element, onVisible)
}

function updateGroup(element, start) {
  const children = new Set(element.children)
  for (const child of groups.get(element) ?? []) {
    if (!children.has(child)) active.get(child)?.cancel()
  }
  groups.set(element, children)
  let position = Number.isFinite(start) ? start : 0
  if (children.size > 1) enqueue(element, 0, 'row')
  for (const child of children) enqueue(child, position++, 'child')
}

export const entrance = {
  mounted(element, { arg, value }) {
    if (arg === 'children') updateGroup(element, value)
    else if (arg === 'after') enqueue(element, 0, 'module', value)
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
