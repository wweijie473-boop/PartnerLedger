<script setup>
import { UiBadge, UiIcon } from '../ui/index.js'
import { statusTone } from '../ui/statusTone.js'

defineProps({ projects: { type: Array, required: true }, showCustomer: Boolean })
defineEmits(['open'])
</script>

<template>
  <div class="project-columns" aria-hidden="true">
    <span>项目</span><span>当前状态</span><span>下一步</span><span>操作</span>
  </div>
  <ul class="project-list" aria-label="项目列表">
    <li v-for="item in projects" :key="item.id">
      <button type="button" class="project-row" :data-project-id="item.id" @click="$emit('open', item)">
        <span class="project-name">
          <strong>{{ item.title }}</strong>
          <small>编号 {{ item.id }}<template v-if="showCustomer && item.customerName"> · {{ item.customerName }}</template><template v-if="item.dueDate"> · 目标日期 {{ item.dueDate }}</template></small>
        </span>
        <UiBadge class="project-status" :tone="statusTone(item.stage)" dot>{{ item.stageLabel }}</UiBadge>
        <span class="project-next">{{ item.nextStep }}</span>
        <span class="project-open">{{ item.isTodo ? '去处理' : '查看' }} <UiIcon name="chevron-right" :size="14" /></span>
      </button>
    </li>
  </ul>
</template>

<style scoped>
.project-columns, .project-row {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(90px, .5fr) minmax(0, 1fr) minmax(64px, auto);
  align-items: center;
  gap: 6px 14px;
  padding: 8px 10px;
}
.project-columns {
  background: var(--ui-surface-muted);
  color: var(--ui-ink-muted);
  font-size: 12px;
}
.project-list { margin: 0; padding: 0; list-style: none; }
.project-row {
  width: 100%;
  border: 0;
  border-top: 1px solid var(--ui-border);
  background: transparent;
  color: var(--ui-ink);
  text-align: left;
}
.project-row:hover, .project-row:focus-visible { background: var(--ui-accent-bg); }
.project-row:focus-visible { outline-offset: -2px; }
.project-name, .project-next { min-width: 0; overflow-wrap: anywhere; }
.project-name strong, .project-name small { display: block; }
.project-name strong { font-weight: 500; }
.project-name small, .project-next { color: var(--ui-ink-secondary); font-size: 12px; }
.project-status { justify-self: start; }
.project-open {
  display: flex;
  gap: 2px;
  align-items: center;
  color: var(--ui-link);
  font-size: 13px;
  white-space: nowrap;
}
@media (max-width: 640px) {
  .project-columns { display: none; }
  .project-row { grid-template-columns: minmax(0, 1fr) auto; padding-inline: 4px; }
  .project-status, .project-open { justify-self: end; }
  .project-status { max-width: min(110px, 33vw); white-space: normal; }
}
</style>
