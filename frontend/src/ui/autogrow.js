import { nextTick } from 'vue'

const fields = new WeakMap()

export const autogrow = {
  mounted(field) {
    let measuredValue
    let measuredWidth

    function resize() {
      const width = field.clientWidth
      const value = field.value
      if (!width || (width === measuredWidth && value === measuredValue)) return

      measuredWidth = width
      measuredValue = value
      field.style.height = ''
      if (!value) return

      const contentHeight = field.scrollHeight
      const initialHeight = field.clientHeight
      if (contentHeight > initialHeight) {
        field.style.height = contentHeight + field.offsetHeight - initialHeight + 'px'
      }
    }

    field.style.overflowY = 'hidden'
    field.addEventListener('input', resize)
    const observer = new ResizeObserver(resize)
    observer.observe(field)
    fields.set(field, { resize, observer })
    // Native v-model can populate a newly mounted field after this directive runs.
    nextTick(resize)
  },
  updated(field) {
    fields.get(field)?.resize()
  },
  beforeUnmount(field) {
    const state = fields.get(field)
    if (!state) return
    field.removeEventListener('input', state.resize)
    state.observer.disconnect()
    fields.delete(field)
  },
}
