<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { UiBadge, UiButton, UiIcon } from '../ui/index.js'
import { aiDemoProject } from '../channel/aiInquiryDemo.js'
import AiApprovalDialog from './AiApprovalDialog.vue'
import { formatBubbleHtml, useAiTheme } from './aiMarkdown.js'
import { skillShortcuts, useAiConversation, workspaceSkills } from './useAiConversation.js'
import { useVoiceInput } from './useVoiceInput.js'

const props = defineProps({
  request: Function,
  demoRepository: Object,
  organizationId: String,
  userId: String,
  organizationName: { type: String, default: '' },
  initialProject: { type: String, default: '' },
})
const emit = defineEmits(['open-project', 'open-projects', 'open-opportunity', 'navigate'])

const log = ref(null)
const composerInput = ref(null)
const skillsExpanded = ref(false)
const sidebarCollapsed = ref(false)
const mobileSidebarOpen = ref(false)
const searchOpen = ref(false)
const sessionQuery = ref('')
const copiedUrl = ref('')

const { theme, toggleTheme } = useAiTheme()

const {
  demo, input, turns, sessions, activeSessionId, projects, selectedId, selectedProject, inquiryProjects,
  loadingProjects, projectError, working, approval, approvalOpen, approvalError, pendingWrite, recoveryError,
  inquiryStepsPreview, applyMarkupToApproval, refreshApprovalMatches, refreshVisitPreview, formatMoneyCents,
  startNewSession, switchSession, deleteSession, clearCurrentSession,
  loadProjects, dispatch, fillManually, closeApproval, reopenApproval, declineApproval, approve, reviewSavedDraft, recoverWrite,
} = useAiConversation(props, log)

const {
  recording, transcribing, autoSending, waveformBars, voiceError,
  startVoice, finishVoice, sendVoice, cancelVoice,
} = useVoiceInput({
  demo,
  request: props.request,
  input,
  onReady: () => nextTick(() => composerInput.value?.focus()),
  onSend: () => dispatch(),
})

const sample = aiDemoProject.requirement + '\n请帮我准备询价草稿。'
const fitLabels = {
  matched: ['符合已知条件', 'success'],
  needs_confirmation: ['待核实', 'warning'],
  mismatch: ['有条件不符', 'danger'],
}
const checkLabels = { met: '符合', unknown: '待核实', missing: '不符' }

const busy = computed(() => Boolean(working.value || approval.value || pendingWrite.value))
const orgLabel = computed(() => props.organizationName || (demo ? '本地演示代理商' : '当前代理商组织'))
const activeSession = computed(() => sessions.value.find(item => item.id === activeSessionId.value) ?? sessions.value[0])

const filteredSessions = computed(() => {
  const kw = sessionQuery.value.trim().toLowerCase()
  if (!kw) return sessions.value
  return sessions.value.filter(sess => sess.title.toLowerCase().includes(kw))
})

const projectSessionGroups = computed(() => {
  const byProject = new Map()
  for (const sess of filteredSessions.value) {
    if (!sess.selectedProjectId) continue
    const proj = projects.value.find(item => String(item.id) === String(sess.selectedProjectId))
    const key = String(sess.selectedProjectId)
    if (!byProject.has(key)) {
      byProject.set(key, {
        projectId: key,
        title: proj?.title || `项目 #${key}`,
        sessions: [],
      })
    }
    byProject.get(key).sessions.push(sess)
  }
  return [...byProject.values()]
})

const recentSessions = computed(() =>
  filteredSessions.value.filter(sess => !sess.selectedProjectId),
)

watch(input, () => {
  nextTick(() => {
    const el = composerInput.value
    if (!el) return
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 168) + 'px'
  })
})

function selectSession(id) {
  switchSession(id)
  mobileSidebarOpen.value = false
}

function handleNewSession() {
  startNewSession()
  mobileSidebarOpen.value = false
  nextTick(() => composerInput.value?.focus())
}

function toggleSidebar() {
  if (window.matchMedia('(max-width: 860px)').matches) {
    mobileSidebarOpen.value = !mobileSidebarOpen.value
  } else {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }
}

function toggleSearch() {
  searchOpen.value = !searchOpen.value
  if (!searchOpen.value) sessionQuery.value = ''
}

function triggerSkill(skill) {
  skillsExpanded.value = false
  dispatch(demo ? skill.demoPrompt : skill.livePrompt)
}

async function copyConfirmLink(url) {
  if (!url || !navigator.clipboard) return
  try {
    await navigator.clipboard.writeText(url)
    copiedUrl.value = url
    setTimeout(() => { if (copiedUrl.value === url) copiedUrl.value = '' }, 2400)
  } catch {}
}

function onEnter(event) {
  if (event.isComposing || event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return
  event.preventDefault()
  skillsExpanded.value = false
  dispatch()
}
</script>

<template>
  <section
    class="ai-codex-shell"
    :class="{ 'is-sidebar-collapsed': sidebarCollapsed, 'is-mobile-sidebar-open': mobileSidebarOpen }"
    :data-ai-theme="theme"
    aria-labelledby="ai-workspace-title"
  >
    <div v-if="mobileSidebarOpen" class="ai-sidebar-backdrop" aria-hidden="true" @click="mobileSidebarOpen = false" />

    <aside class="ai-sidebar" aria-label="AI 会话与项目列表">
      <div class="ai-sidebar-top">
        <div class="ai-brand">
          <span class="ai-brand-dot" aria-hidden="true" />
          <strong>渠智链 AI</strong>
          <small>代理商</small>
        </div>
        <div class="ai-sidebar-tools">
          <button type="button" class="ai-rail-icon-btn" :aria-pressed="searchOpen" title="搜索历史对话" @click="toggleSearch">
            <UiIcon name="search" :size="15" />
          </button>
          <button type="button" class="ai-rail-icon-btn" title="收起侧栏" @click="toggleSidebar">
            <UiIcon name="sidebar-close" :size="15" />
          </button>
        </div>
      </div>

      <div v-if="searchOpen" class="ai-sidebar-search">
        <input v-model="sessionQuery" type="search" placeholder="筛选历史会话…" aria-label="筛选历史会话" />
      </div>

      <button type="button" class="ai-new-chat-btn" :disabled="busy" @click="handleNewSession">
        <UiIcon name="new-chat" :size="15" />
        <span>新对话</span>
      </button>

      <div class="ai-sidebar-scroll" aria-label="历史对话列表">
        <template v-if="projectSessionGroups.length">
          <div v-for="group in projectSessionGroups" :key="group.projectId" class="ai-session-group">
            <div class="ai-folder-head">
              <UiIcon name="folder" :size="14" />
              <span>{{ group.title }}</span>
            </div>
            <div v-for="sess in group.sessions" :key="sess.id" class="ai-session-row" :class="{ 'is-active': sess.id === activeSessionId }">
              <button type="button" class="ai-session-btn" :disabled="busy" @click="selectSession(sess.id)">
                <span>{{ sess.title }}</span>
              </button>
              <button
                v-if="sessions.length > 1 || sess.turns.length"
                type="button"
                class="ai-session-del"
                :disabled="busy"
                aria-label="删除此对话"
                title="删除此对话"
                @click.stop="deleteSession(sess.id)"
              >
                <UiIcon name="close" :size="13" />
              </button>
            </div>
          </div>
        </template>

        <div class="ai-session-section">
          <div class="ai-section-label">最近</div>
          <div
            v-for="sess in (recentSessions.length ? recentSessions : filteredSessions)"
            :key="sess.id"
            class="ai-session-row"
            :class="{ 'is-active': sess.id === activeSessionId }"
          >
            <button type="button" class="ai-session-btn" :disabled="busy" @click="selectSession(sess.id)">
              <span>{{ sess.title }}</span>
            </button>
            <button
              v-if="sessions.length > 1 || sess.turns.length"
              type="button"
              class="ai-session-del"
              :disabled="busy"
              aria-label="删除此对话"
              title="删除此对话"
              @click.stop="deleteSession(sess.id)"
            >
              <UiIcon name="close" :size="13" />
            </button>
          </div>
        </div>
      </div>

      <div class="ai-sidebar-foot">
        <span class="ai-org-pill">
          <UiIcon name="dealer" :size="13" />
          <span>{{ orgLabel }}</span>
        </span>
        <span v-if="demo" class="ai-demo-tag">演示</span>
      </div>
    </aside>

    <div class="ai-stage">
      <header class="ai-topbar">
        <div class="ai-topbar-left">
          <button
            type="button"
            class="ai-topbar-icon-btn ai-open-rail-btn"
            aria-label="展开历史会话侧栏"
            title="展开历史会话侧栏"
            @click="toggleSidebar"
          >
            <UiIcon name="sidebar-open" :size="16" />
          </button>
          <h1 id="ai-workspace-title" class="ai-topbar-title">{{ turns.length ? (activeSession?.title || 'AI 工作台') : 'AI 工作台' }}</h1>
          <span v-if="selectedProject" class="ai-topbar-project-badge">
            <UiIcon name="folder" :size="12" />
            {{ selectedProject.title }}
          </span>
        </div>

        <div class="ai-topbar-right">
          <button
            type="button"
            class="ai-topbar-pill"
            :aria-expanded="skillsExpanded"
            @click="skillsExpanded = !skillsExpanded"
          >
            <UiIcon name="workflow" :size="14" />
            <span>技能 ({{ skillShortcuts.length }})</span>
          </button>
          <button
            v-if="turns.length"
            type="button"
            class="ai-topbar-icon-btn"
            :disabled="busy"
            title="清空当前对话"
            aria-label="清空当前对话"
            @click="clearCurrentSession"
          >
            <UiIcon name="trash" :size="15" />
          </button>
          <button
            type="button"
            class="ai-topbar-icon-btn"
            :title="theme === 'dark' ? '切换为极简浅色' : '切换为 Codex 深色'"
            :aria-label="theme === 'dark' ? '切换为极简浅色' : '切换为 Codex 深色'"
            @click="toggleTheme"
          >
            <UiIcon :name="theme === 'dark' ? 'sun' : 'moon'" :size="15" />
          </button>
        </div>
      </header>

      <div ref="log" class="ai-log" :class="{ 'is-empty': !turns.length }" role="log" aria-label="AI 对话记录">
        <div v-if="!turns.length" class="ai-empty-hero">
          <div class="ai-codex-emblem" aria-hidden="true">
            <svg viewBox="0 0 48 48" width="46" height="46" fill="none">
              <path
                d="M24 6c3.2 0 5.9 1.7 7.3 4.3 2.9-.5 5.9.8 7.4 3.4 1.5 2.6 1.1 5.8-.8 8 1.9 2.2 2.3 5.4.8 8-1.5 2.6-4.5 3.9-7.4 3.4-1.4 2.6-4.1 4.3-7.3 4.3s-5.9-1.7-7.3-4.3c-2.9.5-5.9-.8-7.4-3.4-1.5-2.6-1.1-5.8.8-8-1.9-2.2-2.3-5.4-.8-8 1.5-2.6 4.5-3.9 7.4-3.4C18.1 7.7 20.8 6 24 6Z"
                stroke="currentColor"
                stroke-width="2"
                stroke-linejoin="round"
              />
              <path d="M18.5 20.5L22.5 24l-4 3.5M24.5 27.5h5" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </div>
          <h2 class="ai-empty-title">今天想推进哪项业务？</h2>
          <p class="ai-empty-sub">自由提问分析客户与经营台账，或直接下达指令自动打包审批执行。</p>

          <div class="ai-starter-chips">
            <button
              v-if="demo"
              type="button"
              class="ai-starter-chip is-accent"
              :disabled="loadingProjects || !!working || !!pendingWrite"
              @click="dispatch(sample)"
            >
              <UiIcon name="ai" :size="14" />
              <span>试试 ERP 询价示例</span>
            </button>
            <button
              v-else
              type="button"
              class="ai-starter-chip is-accent"
              :disabled="loadingProjects || !!working || !!pendingWrite"
              @click="dispatch('帮我梳理名下客户档案，并分析近期最容易成交续费或增购的客户')"
            >
              <UiIcon name="ai" :size="14" />
              <span>分析客户与续费机会</span>
            </button>
            <button
              v-for="skill in skillShortcuts.slice(1, 5)"
              :key="skill.id"
              type="button"
              class="ai-starter-chip"
              :disabled="loadingProjects || !!working || !!pendingWrite"
              @click="triggerSkill(skill)"
            >
              <UiIcon :name="skill.icon" :size="14" />
              <span>{{ skill.label }}</span>
            </button>
            <button
              type="button"
              class="ai-starter-chip"
              :disabled="loadingProjects || !!working || !!pendingWrite"
              @click="dispatch('帮我查一下当前项目进展')"
            >
              <UiIcon name="activity" :size="14" />
              <span>查看项目进展</span>
            </button>
          </div>
        </div>

        <div v-else class="ai-stream-column">
          <div v-for="turn in turns" :key="turn.id" class="ai-turn" :class="{ 'is-user': turn.role === 'user' }">
            <template v-if="turn.role === 'user'">
              <div class="ai-user-bubble">{{ turn.text }}</div>
            </template>
            <template v-else>
              <div class="ai-assistant-row">
                <div class="ai-assistant-badge" aria-hidden="true"><UiIcon name="ai" :size="15" /></div>
                <div class="ai-turn-content">
                  <div class="ai-prose" :class="{ 'is-streaming': turn.streaming }" v-html="formatBubbleHtml(turn.text)" />

                  <button
                    v-if="turn.manualSource"
                    type="button"
                    class="ai-action-chip"
                    :disabled="busy"
                    @click="fillManually(turn.manualSource, turn.fallbackIntent)"
                  >
                    <UiIcon name="permission" :size="14" />
                    <span>手工打开审批单继续办理</span>
                  </button>

                  <ul v-if="turn.stepsLog?.length" class="ai-executed-steps" aria-label="已自动完成的步骤">
                    <li v-for="(stepLabel, idx) in turn.stepsLog" :key="idx">
                      <UiIcon name="check" :size="13" />
                      <span>{{ stepLabel }}</span>
                    </li>
                  </ul>

                  <div v-if="turn.approval" class="ai-result">
                    <div class="ai-result-head">
                      <strong>{{ turn.approval.title }}</strong>
                      <UiBadge :tone="approval === turn.approval ? 'warning' : 'neutral'">
                        {{ approval === turn.approval ? '待人工审批' : '审批已结束' }}
                      </UiBadge>
                    </div>
                    <button v-if="approval === turn.approval" type="button" class="ai-action-chip is-primary" @click="reopenApproval">
                      <UiIcon name="permission" :size="14" />
                      <span>打开审批弹窗</span>
                    </button>
                  </div>

                  <div v-if="turn.matches?.length" class="ai-results">
                    <article v-for="item in turn.matches" :key="item.product.id" class="ai-result">
                      <div class="ai-result-head">
                        <strong>{{ item.product.name }} {{ item.product.version }}</strong>
                        <UiBadge :tone="fitLabels[item.status]?.[1]">{{ fitLabels[item.status]?.[0] ?? '待核实' }}</UiBadge>
                      </div>
                      <ul v-if="item.checks?.length" class="ai-checks">
                        <li v-for="(check, index) in item.checks" :key="index">
                          <span :data-check="check.status">{{ checkLabels[check.status] }}</span>
                          <span>{{ check.value ? check.value + ' · ' : '' }}{{ check.detail }}</span>
                        </li>
                      </ul>
                      <details v-if="item.sources?.length">
                        <summary>查看厂商资料</summary>
                        <div v-for="source in item.sources" :key="source.title + source.reference" class="ai-source">
                          <strong>{{ source.title }}</strong> · {{ source.reference }}<p>{{ source.excerpt }}</p>
                        </div>
                      </details>
                    </article>
                  </div>

                  <article v-if="turn.draft" class="ai-result">
                    <div class="ai-result-head">
                      <strong>询价草稿 <template v-if="turn.product">· {{ turn.product.name }}</template></strong>
                      <UiBadge tone="success">已保存</UiBadge>
                    </div>
                    <p>{{ turn.draft.summary }}</p>
                    <div class="ai-prompts">
                      <button type="button" class="ai-action-chip" @click="emit('open-project', turn.draft.projectId)">
                        <span>打开项目</span>
                        <UiIcon name="open-detail" :size="13" />
                      </button>
                      <button type="button" class="ai-action-chip is-primary" :disabled="busy" @click="reviewSavedDraft(turn)">
                        <UiIcon name="permission" :size="13" />
                        <span>审批发送询价</span>
                      </button>
                    </div>
                  </article>

                  <article v-if="turn.project" class="ai-result">
                    <div class="ai-result-head">
                      <strong>{{ turn.project.title }}</strong>
                      <UiBadge tone="info">{{ turn.project.stageLabel }}</UiBadge>
                    </div>
                    <p v-if="turn.project.order?.id">正式订单号：{{ turn.project.order.id }}</p>
                    <p v-if="turn.project.nextStep">下一步：{{ turn.project.nextStep }}</p>
                    <button type="button" class="ai-action-chip" @click="emit('open-project', turn.project.id)">
                      <span>打开项目</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </article>

                  <article v-if="turn.opportunityLink" class="ai-result">
                    <div class="ai-result-head">
                      <strong>团队附加商机</strong>
                      <UiBadge tone="success">已沉淀</UiBadge>
                    </div>
                    <p>{{ turn.opportunityLink.note }}</p>
                    <div class="ai-prompts">
                      <button type="button" class="ai-action-chip" @click="emit('open-opportunity', turn.opportunityLink.id, turn.opportunityLink.projectId)">
                        <span>打开商机拾遗</span>
                        <UiIcon name="open-detail" :size="13" />
                      </button>
                      <button v-if="turn.opportunityLink.projectId" type="button" class="ai-action-chip" @click="emit('open-project', turn.opportunityLink.projectId)">
                        <span>查看关联主项目</span>
                      </button>
                    </div>
                  </article>

                  <article v-if="turn.customersCard" class="ai-result ai-data-card">
                    <ul class="ai-entity-list">
                      <li v-for="cust in turn.customersCard.items.slice(0, 12)" :key="cust.id">
                        <div class="ai-result-head">
                          <strong>{{ cust.name }}</strong>
                          <UiBadge :tone="cust.linked ? 'success' : cust.prospect ? 'warning' : 'info'">
                            {{ cust.linked ? '已关联' : cust.prospect ? '潜在客户' : '直购客户' }}
                          </UiBadge>
                        </div>
                        <small class="ai-entity-meta">联系人：{{ cust.contact || '未填写' }}<template v-if="cust.note"> · {{ cust.note }}</template></small>
                      </li>
                    </ul>
                    <div class="ai-prompts">
                      <button type="button" class="ai-action-chip" @click="emit('navigate', 'customers')">
                        <span>打开客户档案</span>
                        <UiIcon name="open-detail" :size="13" />
                      </button>
                      <button type="button" class="ai-action-chip" @click="emit('navigate', 'assisted')">
                        <span>去代客下单</span>
                      </button>
                    </div>
                  </article>

                  <article v-if="turn.operationsCard" class="ai-result ai-data-card">
                    <div class="ai-mini-metrics">
                      <div><span>客户档案</span><strong>{{ turn.operationsCard.customers }}</strong></div>
                      <div><span>直购订单</span><strong>{{ turn.operationsCard.orders }}</strong></div>
                      <div><span>待结佣金</span><strong>{{ turn.operationsCard.pendingCommissionText }}</strong></div>
                      <div><span>30天内续费</span><strong>{{ turn.operationsCard.renewals30Days }}</strong></div>
                      <div><span>未关闭工单</span><strong>{{ turn.operationsCard.openTickets }}</strong></div>
                    </div>
                    <button type="button" class="ai-action-chip" @click="emit('navigate', 'business-overview')">
                      <span>打开经营看板</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </article>

                  <article v-if="turn.projectsList?.length" class="ai-result ai-data-card">
                    <ul class="ai-entity-list">
                      <li v-for="item in turn.projectsList" :key="item.id">
                        <div class="ai-result-head">
                          <strong>{{ item.title }}</strong>
                          <UiBadge :tone="item.isTodo ? 'warning' : 'info'">{{ item.stageLabel }}</UiBadge>
                        </div>
                        <div class="ai-entity-row">
                          <small class="ai-entity-meta">下一步：{{ item.nextStep }}</small>
                          <button type="button" class="ai-action-chip" @click="emit('open-project', item.id)">
                            <span>打开</span>
                            <UiIcon name="open-detail" :size="13" />
                          </button>
                        </div>
                      </li>
                    </ul>
                  </article>

                  <div v-if="turn.chatLinks?.length" class="ai-prompts">
                    <button
                      v-for="link in turn.chatLinks"
                      :key="link.view"
                      type="button"
                      class="ai-action-chip"
                      @click="link.view === 'projects' ? emit('open-projects') : link.view === 'opportunities' ? emit('open-opportunity', '', '') : emit('navigate', link.view)"
                    >
                      <span>{{ link.label }}</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </div>

                  <article v-if="turn.navAction" class="ai-result">
                    <div class="ai-prompts">
                      <button type="button" class="ai-action-chip" @click="emit('navigate', turn.navAction.view)">
                        <span>{{ turn.navAction.label }}</span>
                        <UiIcon name="open-detail" :size="13" />
                      </button>
                      <button v-if="turn.navAction.confirmUrl" type="button" class="ai-action-chip" @click="copyConfirmLink(turn.navAction.confirmUrl)">
                        <span>{{ copiedUrl === turn.navAction.confirmUrl ? '确认链接已复制' : '复制客户确认链接' }}</span>
                      </button>
                    </div>
                  </article>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>

      <div class="ai-dock">
        <div class="ai-dock-inner">
          <section v-if="skillsExpanded" class="ai-skill-popover" aria-label="已集成工作流技能">
            <div class="ai-skill-popover-head">
              <strong>选择要调用的业务工作流技能</strong>
              <button type="button" class="ai-rail-icon-btn" aria-label="关闭技能菜单" @click="skillsExpanded = false">
                <UiIcon name="close" :size="14" />
              </button>
            </div>
            <div class="ai-skill-grid">
              <button
                v-for="skill in skillShortcuts"
                :key="skill.id"
                type="button"
                class="ai-skill-card"
                :disabled="loadingProjects || !!working || !!pendingWrite"
                @click="triggerSkill(skill)"
              >
                <span class="ai-skill-card-head">
                  <UiIcon :name="skill.icon" :size="15" />
                  <strong>{{ skill.label }}</strong>
                </span>
                <small>{{ skill.description }}</small>
              </button>
            </div>
          </section>

          <div class="ai-context-strip">
            <div class="ai-context-items">
              <span class="ai-context-chip">
                <UiIcon name="dealer" :size="13" />
                <span>{{ orgLabel }}</span>
              </span>
              <label class="ai-context-select-wrap">
                <UiIcon name="folder" :size="13" />
                <select
                  v-model="selectedId"
                  aria-label="项目上下文"
                  class="ai-context-select"
                  :disabled="!!working || !!approval || loadingProjects"
                >
                  <option value="">选择项目（自动识别）</option>
                  <option v-for="project in projects" :key="project.id" :value="project.id">
                    {{ project.title }} · {{ project.id }}
                  </option>
                </select>
              </label>
            </div>

            <div v-if="working" class="ai-working-inline" role="status">
              <UiIcon name="loading" :size="13" class="ai-spin" />
              <span>{{ working }}…</span>
            </div>
            <div v-else-if="projectError" class="ai-context-error" role="status">
              <span>{{ projectError }}</span>
              <button type="button" :disabled="loadingProjects" @click="loadProjects">重试</button>
            </div>
          </div>

          <form
            class="ai-composer-pod"
            :class="{ 'is-voice-mode': recording || transcribing }"
            @submit.prevent="dispatch()"
          >
            <template v-if="recording || transcribing">
              <div class="ai-voice-capsule" role="status" :aria-label="transcribing ? '正在转写语音' : '正在录音'">
                <button
                  type="button"
                  class="ai-voice-circle-btn is-cancel"
                  aria-label="取消录音"
                  title="取消录音"
                  @click="cancelVoice"
                >
                  <UiIcon name="close" :size="16" />
                </button>

                <div class="ai-waveform-track" :class="{ 'is-transcribing': transcribing }" aria-hidden="true">
                  <span
                    v-for="(lvl, idx) in waveformBars"
                    :key="idx"
                    class="ai-wave-bar"
                    :class="{ 'is-active': lvl > 0.02 }"
                    :style="{ height: `${lvl > 0.02 ? Math.round(4 + lvl * 22) : 2}px` }"
                  />
                </div>

                <div class="ai-voice-actions">
                  <button
                    type="button"
                    class="ai-voice-circle-btn is-stop"
                    aria-label="完成说词并转写"
                    title="停止录音并转为文字编辑"
                    :disabled="transcribing"
                    @click="finishVoice"
                  >
                    <UiIcon v-if="transcribing && !autoSending" name="loading" :size="15" class="ai-spin" />
                    <span v-else class="ai-stop-square" aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    class="ai-voice-circle-btn is-send"
                    aria-label="完成录音并直接发送"
                    title="停止录音并直接发送"
                    :disabled="transcribing"
                    @click="sendVoice"
                  >
                    <UiIcon v-if="transcribing && autoSending" name="loading" :size="15" class="ai-spin" />
                    <UiIcon v-else name="send-up" :size="16" />
                  </button>
                </div>
              </div>
            </template>

            <template v-else>
              <p v-if="voiceError" class="ai-voice-error" role="alert">{{ voiceError }}</p>
              <div v-if="pendingWrite && !working" class="ai-recovery-banner" role="status">
                <span>上次工作流结果待确认。{{ recoveryError }}</span>
                <UiButton compact :disabled="!!working" @click="recoverWrite">用原请求确认结果</UiButton>
              </div>

              <textarea
                ref="composerInput"
                v-model="input"
                aria-label="给渠道业务助手发消息"
                rows="1"
                maxlength="2000"
                placeholder="随心输入业务指令、客户分析问题，或点击右侧语音输入…"
                :disabled="!!pendingWrite"
                @keydown.enter="onEnter"
              />

              <div class="ai-composer-bar">
                <div class="ai-composer-left">
                  <button
                    type="button"
                    class="ai-plus-trigger"
                    :aria-expanded="skillsExpanded"
                    title="打开业务工作流技能面板"
                    @click="skillsExpanded = !skillsExpanded"
                  >
                    <UiIcon name="add" :size="15" />
                  </button>
                  <span class="ai-guard-badge">
                    <UiIcon name="shield-check" :size="13" />
                    <span>弹窗审批执行</span>
                  </span>
                </div>

                <div class="ai-composer-right">
                  <span class="ai-model-label">{{ demo ? '本地演示引擎' : '渠智链多模池 · 自动轮换' }}</span>
                  <button
                    type="button"
                    class="ai-mic-btn"
                    aria-label="语音输入"
                    title="语音输入"
                    :disabled="!!working || loadingProjects || !!pendingWrite"
                    @click="startVoice"
                  >
                    <UiIcon name="mic" :size="15" />
                  </button>
                  <button
                    type="submit"
                    class="ai-send-btn"
                    aria-label="发送"
                    title="发送消息"
                    :disabled="!input.trim() || !!working || loadingProjects || !!pendingWrite"
                  >
                    <UiIcon name="send-up" :size="16" />
                  </button>
                </div>
              </div>
            </template>
          </form>
        </div>
      </div>
    </div>

    <AiApprovalDialog
      :open="approvalOpen"
      :approval="approval"
      :error="approvalError"
      :working="working"
      :demo="demo"
      :projects="projects"
      :inquiry-projects="inquiryProjects"
      :skill-labels="workspaceSkills"
      :inquiry-steps-preview="inquiryStepsPreview"
      :format-money-cents="formatMoneyCents"
      @close="closeApproval"
      @decline="declineApproval"
      @approve="approve"
      @refresh-matches="refreshApprovalMatches"
      @apply-markup="applyMarkupToApproval(approval)"
      @refresh-visit="refreshVisitPreview"
      @open-projects="emit('open-projects')"
    />
  </section>
</template>

<style src="./ai-workspace.css"></style>
