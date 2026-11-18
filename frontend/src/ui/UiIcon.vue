<script setup>
import { icons } from './icons.js'

defineProps({
  name: { type: String, required: true, validator: (name) => Object.hasOwn(icons, name) },
  size: { type: Number, default: 18 },
  label: { type: String, default: '' },
})
</script>

<template>
  <component v-if="icons[name]" :is="icons[name]" :size="size" :stroke-width="1.75"
    :role="label ? 'img' : undefined" :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'" focusable="false" />
</template>
