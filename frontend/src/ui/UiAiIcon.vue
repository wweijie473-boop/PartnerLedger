<script setup>
defineProps({
  size: { type: Number, default: 24 },
  strokeWidth: { type: Number, default: 1.6 },
})
</script>

<template>
  <svg xmlns="http://www.w3.org/2000/svg" :width="size" :height="size" viewBox="0 0 24 24"
    fill="none" stroke="currentColor" :stroke-width="strokeWidth" stroke-linecap="round" stroke-linejoin="round">
    <path d="M10.5 2.5 12.6 8.4l5.9 2.1-5.9 2.1-2.1 5.9-2.1-5.9-5.9-2.1 5.9-2.1Z" fill="currentColor" fill-opacity=".12" />
    <path d="M19.5 2.5v6m-3-3h6M20 14.5v4m-2-2h4M5 16.5v4m-2-2h4" />
  </svg>
</template>
