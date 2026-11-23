<script setup>
import { UiIcon } from './ui/index.js'

defineProps({
  label: { type: String, required: true },
  organization: { type: String, required: true },
  href: { type: String, default: '' },
  logo: { type: String, default: '' },
})
</script>

<template>
  <component :is="href ? 'a' : 'button'" class="workspace-account" :href="href || undefined"
    :type="href ? undefined : 'button'" :aria-label="'账号：' + label + '，' + organization"
    :title="label + ' · ' + organization">
    <span v-if="logo" class="workspace-avatar workspace-avatar-logo" aria-hidden="true">
      <img :src="logo" width="22" height="22" alt="" />
    </span>
    <span v-else class="workspace-avatar" aria-hidden="true">{{ [...organization][0] ?? '渠' }}</span>
    <span class="workspace-account-text">
      <strong>{{ label }}</strong>
      <small>{{ organization }}</small>
    </span>
    <UiIcon name="chevron-right" :size="14" />
  </component>
</template>

<style scoped>
.workspace-account {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 34px;
  padding: 3px 6px;
  border: 0;
  border-radius: var(--ui-radius-sm);
  background: transparent;
  color: var(--ui-ink);
  text-align: left;
  text-decoration: none;
}
.workspace-account:hover, .workspace-account:focus-visible {
  background: var(--ui-accent-bg);
  color: var(--ui-link);
}
.workspace-avatar {
  display: grid;
  width: 26px;
  height: 26px;
  flex: none;
  place-items: center;
  border-radius: 50%;
  background: var(--ui-accent-bg);
  color: var(--ui-link);
  font-size: 13px;
  font-weight: 600;
}
.workspace-avatar-logo {
  overflow: hidden;
  background: #fff;
}
.workspace-avatar-logo img {
  object-fit: contain;
  /* Account for the artwork's uneven white margins. */
  transform: translate(9.2%, 3.4%);
}
.workspace-account-text { min-width: 0; max-width: clamp(84px, 13vw, 136px); }
.workspace-account-text > * {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.25;
}
.workspace-account-text strong { font-size: 13px; font-weight: 500; }
.workspace-account-text small { font-size: 11px; color: var(--ui-ink-muted); }
.workspace-account > svg { color: var(--ui-ink-muted); }
@media (max-width: 760px) {
  .workspace-account { min-height: 38px; }
  .workspace-account-text { max-width: min(125px, 29vw); }
}
</style>
