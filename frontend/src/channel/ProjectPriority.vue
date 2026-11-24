<script setup>
import ProjectList from './ProjectList.vue'
import { UiButton, UiState } from '../ui/index.js'

defineProps({ projects: { type: Array, required: true }, showCustomer: Boolean })
defineEmits(['open', 'show-all'])
</script>

<template>
  <section class="workspace-section" aria-labelledby="project-tasks">
    <header class="workspace-section-heading">
      <h2 id="project-tasks">项目待办</h2>
      <UiButton v-if="projects.length" variant="text" end-icon="forward" @click="$emit('show-all')">查看全部待办</UiButton>
    </header>
    <ProjectList v-if="projects.length" :projects="projects.slice(0, 5)" :show-customer="showCustomer" @open="$emit('open', $event)" />
    <UiState v-else title="当前没有待办理项目" />
  </section>
</template>
