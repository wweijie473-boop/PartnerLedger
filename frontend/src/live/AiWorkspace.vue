<script setup>
import { ref } from 'vue'
import { UiBadge, UiButton, UiField, UiIcon } from '../ui/index.js'
import { aiDemoProject } from '../channel/aiInquiryDemo.js'
import AiApprovalDialog from './AiApprovalDialog.vue'
import { skillShortcuts, useAiConversation, workspaceSkills } from './useAiConversation.js'

const props = defineProps({
  request: Function,
  demoRepository: Object,
  organizationId: String,
  userId: String,
  initialProject: { type: String, default: '' },
})
const emit = defineEmits(['open-project', 'open-projects', 'open-opportunity', 'navigate'])
const log = ref(null)
const skillsExpanded = ref(false)
const copiedUrl = ref('')

const {
  demo, input, turns, projects, selectedId, inquiryProjects, loadingProjects, projectError, working,
  approval, approvalOpen, approvalError, pendingWrite, recoveryError,
  inquiryStepsPreview, applyMarkupToApproval, refreshApprovalMatches, refreshVisitPreview, formatMoneyCents,
  loadProjects, dispatch, fillManually, closeApproval, reopenApproval, declineApproval, approve, reviewSavedDraft, recoverWrite,
} = useAiConversation(props, log)

const sample = aiDemoProject.requirement + '\n请帮我准备询价草稿。'
const fitLabels = {
  matched: ['符合已知条件', 'success'],
  needs_confirmation: ['待核实', 'warning'],
  mismatch: ['有条件不符', 'danger'],
}
const checkLabels = { met: '符合', unknown: '待核实', missing: '不符' }

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
  dispatch()
}
</script>

<template>
  <section class="ai-workspace" aria-labelledby="ai-workspace-title">
    <header class="ai-hero">
      <div class="ai-hero-heading">
        <div class="ai-emblem"><UiIcon name="ai" :size="36" /></div>
        <div>
          <h1 id="ai-workspace-title">AI 工作台</h1>
          <p>自然语言驱动全业务工作流：自动识别技能、弹窗打包审批、同意后全自动执行到底。</p>
        </div>
      </div>
      <div class="ai-hero-badges">
        <UiButton compact variant="secondary" icon="workflow" :aria-expanded="skillsExpanded" @click="skillsExpanded = !skillsExpanded">
          {{ skillsExpanded ? '收起技能面板' : `已接入技能 (${skillShortcuts.length})` }}
        </UiButton>
        <UiBadge v-if="demo" tone="info">本地样例</UiBadge>
      </div>
    </header>

    <section v-if="skillsExpanded" class="ai-skill-drawer" aria-label="已集成工作流技能">
      <button
        v-for="skill in skillShortcuts"
        :key="skill.id"
        type="button"
        class="ai-skill-card"
        :disabled="loadingProjects || !!working || !!pendingWrite"
        @click="triggerSkill(skill)"
      >
        <span class="ai-skill-card-head">
          <UiIcon :name="skill.icon" :size="16" />
          <strong>{{ skill.label }}</strong>
        </span>
        <small>{{ skill.description }}</small>
      </button>
    </section>

    <div class="ai-chat">
      <div class="ai-chat-head">
        <div class="ai-assistant-title"><UiIcon name="ai" :size="20" /><strong>渠道业务工作流助手</strong></div>
        <UiField label="项目上下文" v-slot="{ control }">
          <select v-bind="control" v-model="selectedId" :disabled="!!working || !!approval || loadingProjects">
            <option value="">自动识别或新建项目（可选指定）</option>
            <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.title }} · {{ project.id }}</option>
          </select>
        </UiField>
      </div>
      <p v-if="projectError" class="ai-notice" role="status">
        {{ projectError }} <UiButton compact variant="text" :disabled="loadingProjects" @click="loadProjects">刷新项目</UiButton>
      </p>

      <div ref="log" class="ai-log" role="log" aria-label="AI 对话记录">
        <div class="ai-turn">
          <div class="ai-avatar"><UiIcon name="ai" :size="19" /></div>
          <div class="ai-turn-content">
            <p class="ai-speaker">渠道业务助手</p>
            <p class="ai-bubble">直接告诉我你想办理什么业务（如建项询价、对客报价、确认下单、提交验收、记拜访转商机、代客直购、催结佣金、开票或提工单）。我会自动拉取数据并弹出打包审批单，你点击同意后我将自动完成后续全部操作。</p>
            <div v-if="!turns.length" class="ai-prompts">
              <UiButton v-if="demo" compact icon="ai" :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch(sample)">试试 ERP 询价示例</UiButton>
              <UiButton v-else compact icon="ai" :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch('根据当前项目的采购需求准备询价草稿')">整理当前项目需求</UiButton>
              <UiButton
                v-for="skill in skillShortcuts.slice(1, 5)"
                :key="skill.id"
                compact
                :icon="skill.icon"
                :disabled="loadingProjects || !!working || !!pendingWrite"
                @click="triggerSkill(skill)"
              >
                {{ skill.label }}
              </UiButton>
              <UiButton compact :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch('帮我查一下当前项目进展')">查看项目进展</UiButton>
            </div>
          </div>
        </div>

        <div v-for="turn in turns" :key="turn.id" class="ai-turn" :class="{ 'is-user': turn.role === 'user' }">
          <div v-if="turn.role === 'assistant'" class="ai-avatar"><UiIcon name="ai" :size="19" /></div>
          <div class="ai-turn-content">
            <p class="ai-speaker">{{ turn.role === 'user' ? '你' : '渠道业务助手' }}</p>
            <p class="ai-bubble">{{ turn.text }}</p>

            <UiButton
              v-if="turn.manualSource"
              compact
              icon="permission"
              :disabled="!!working || !!approval || !!pendingWrite"
              @click="fillManually(turn.manualSource, turn.fallbackIntent)"
            >
              手工打开审批单继续办理
            </UiButton>

            <ul v-if="turn.stepsLog?.length" class="ai-executed-steps" aria-label="已自动完成的步骤">
              <li v-for="(stepLabel, idx) in turn.stepsLog" :key="idx">
                <UiIcon name="check" :size="14" />
                <span>{{ stepLabel }}</span>
              </li>
            </ul>

            <div v-if="turn.approval" class="ai-result">
              <div class="ai-result-head">
                <strong>{{ turn.approval.title }}</strong>
                <UiBadge :tone="approval === turn.approval ? 'warning' : 'neutral'">{{ approval === turn.approval ? '待人工审批' : '审批已结束' }}</UiBadge>
              </div>
              <UiButton v-if="approval === turn.approval" compact icon="permission" @click="reopenApproval">打开审批弹窗</UiButton>
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
                <UiButton compact end-icon="open-detail" @click="emit('open-project', turn.draft.projectId)">打开项目</UiButton>
                <UiButton compact icon="permission" :disabled="!!working || !!approval || !!pendingWrite" @click="reviewSavedDraft(turn)">审批发送询价</UiButton>
              </div>
            </article>

            <article v-if="turn.project" class="ai-result">
              <div class="ai-result-head">
                <strong>{{ turn.project.title }}</strong>
                <UiBadge tone="info">{{ turn.project.stageLabel }}</UiBadge>
              </div>
              <p v-if="turn.project.order?.id">正式订单号：{{ turn.project.order.id }}</p>
              <p v-if="turn.project.nextStep">下一步：{{ turn.project.nextStep }}</p>
              <UiButton compact end-icon="open-detail" @click="emit('open-project', turn.project.id)">打开项目</UiButton>
            </article>

            <article v-if="turn.opportunityLink" class="ai-result">
              <div class="ai-result-head">
                <strong>团队附加商机</strong>
                <UiBadge tone="success">已沉淀</UiBadge>
              </div>
              <p>{{ turn.opportunityLink.note }}</p>
              <div class="ai-prompts">
                <UiButton compact end-icon="open-detail" @click="emit('open-opportunity', turn.opportunityLink.id, turn.opportunityLink.projectId)">打开商机拾遗</UiButton>
                <UiButton v-if="turn.opportunityLink.projectId" compact variant="text" @click="emit('open-project', turn.opportunityLink.projectId)">查看关联主项目</UiButton>
              </div>
            </article>

            <article v-if="turn.navAction" class="ai-result">
              <div class="ai-prompts">
                <UiButton compact end-icon="open-detail" @click="emit('navigate', turn.navAction.view)">{{ turn.navAction.label }}</UiButton>
                <UiButton v-if="turn.navAction.confirmUrl" compact @click="copyConfirmLink(turn.navAction.confirmUrl)">
                  {{ copiedUrl === turn.navAction.confirmUrl ? '确认链接已复制' : '复制客户确认链接' }}
                </UiButton>
              </div>
            </article>
          </div>
        </div>
      </div>

      <div class="ai-composer">
        <div v-if="working" class="ai-working" role="status"><UiIcon name="loading" :size="16" /> {{ working }}…</div>
        <div v-if="pendingWrite && !working" class="ai-recovery" role="status">
          <span>上次工作流结果待确认。{{ recoveryError }}</span>
          <UiButton compact :disabled="!!working" @click="recoverWrite">用原请求确认结果</UiButton>
        </div>
        <form class="ai-composer-form" @submit.prevent="dispatch()">
          <textarea
            v-model="input"
            aria-label="给渠道业务助手发消息"
            rows="2"
            maxlength="2000"
            placeholder="输入你想办的业务，例如：给财务共享中心加价 25% 发对客报价 / 记拜访转商机 / 代客下单 2 套云财务"
            :disabled="!!pendingWrite"
            @keydown.enter="onEnter"
          />
          <UiButton type="submit" variant="primary" end-icon="forward" :disabled="!input.trim() || !!working || loadingProjects || !!pendingWrite">发送</UiButton>
        </form>
        <p class="ai-composer-hint">Enter 发送 · Shift + Enter 换行 · 所有写操作均先弹出打包审批单，同意后自动执行到底</p>
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
