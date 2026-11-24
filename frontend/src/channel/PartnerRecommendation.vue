<script setup>
import { computed, ref } from 'vue'
import { UiButton } from '../ui/index.js'

const props = defineProps({ candidate: {type:Object, required:true}, brief: {type:String, required:true} })
const message = computed(() => `${props.candidate.owner}，你好！想请你帮忙问一下${props.candidate.name}。${props.brief}`)
const feedback = ref('')
const manualCopy = ref(false)

async function copyInquiry() {
  manualCopy.value = true
  try {
    await navigator.clipboard.writeText(message.value)
    manualCopy.value = false
    feedback.value = '已复制，可到微信粘贴。'
  } catch {
    feedback.value = '请手动复制下方文案。'
  }
}
</script>

<template>
  <article class="partner-recommendation">
    <h3>找 {{ candidate.owner }} 引荐 {{ candidate.name }}</h3>
    <p class="muted">{{ candidate.reason }}</p>
    <p v-if="candidate.evidence">{{ candidate.evidence }}</p>
    <p v-if="candidate.caveat" class="muted">{{ candidate.caveat }}</p>
    <UiButton compact variant="primary" @click="copyInquiry">复制微信询问</UiButton>
    <p v-if="feedback" role="status">{{ feedback }}</p>
    <textarea v-autogrow v-if="manualCopy" aria-label="微信询问文案" :value="message" readonly rows="3"></textarea>
    <slot />
  </article>
</template>

<style scoped>
.partner-recommendation { display:grid; justify-items:start; gap:8px; padding-block:14px; overflow-wrap:anywhere; }
.partner-recommendation + .partner-recommendation { border-top:1px solid var(--ui-border); }
.partner-recommendation :is(h3,p) { margin:0; }
.partner-recommendation textarea { width:100%; }
</style>
