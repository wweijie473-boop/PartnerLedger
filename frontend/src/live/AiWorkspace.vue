<script setup>
import { ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiField, UiIcon } from '../ui/index.js'
import { aiDemoProject } from '../channel/aiInquiryDemo.js'
import { useAiConversation, workspaceSkills } from './useAiConversation.js'

const props = defineProps({
  request: Function,
  demoRepository: Object,
  organizationId: String,
  userId: String,
  initialProject: { type: String, default: '' },
})
const emit = defineEmits(['open-project', 'open-projects'])
const log = ref(null)
const {
  demo, input, turns, projects, selectedId, inquiryProjects, loadingProjects, projectError, working,
  approval, approvalOpen, approvalError, choices, pendingWrite, recoveryError,
  loadProjects, dispatch, fillManually, closeApproval, reopenApproval, declineApproval, approve, useCandidate, reviewSavedDraft, recoverWrite,
} = useAiConversation(props, log)

const sample = aiDemoProject.requirement + '\n请帮我准备询价草稿。'
const fields = [
  { key: 'enterprise_type', label: '企业类型', max: 120 },
  { key: 'employee_count', label: '员工人数', numeric: true },
  { key: 'required_modules', label: '所需模块', hint: '多个模块用顿号或逗号分隔。' },
  { key: 'deployment_mode', label: '部署方式', max: 80 },
  { key: 'license_count', label: '授权用户数', numeric: true },
  { key: 'version', label: '指定版本', max: 100 },
]
const fitLabels = {
  matched: ['符合已知条件', 'success'],
  needs_confirmation: ['待核实', 'warning'],
  mismatch: ['有条件不符', 'danger'],
}
const checkLabels = { met: '符合', unknown: '待核实', missing: '不符' }

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
        <div><h1 id="ai-workspace-title">AI 工作台</h1><p>告诉我你想办什么，我会调用对应技能。</p></div>
      </div>
      <UiBadge v-if="demo" tone="info">本地样例</UiBadge>
    </header>

    <div class="ai-chat">
      <div class="ai-chat-head">
        <div class="ai-assistant-title"><UiIcon name="ai" :size="20" /><strong>渠道业务助手</strong></div>
        <UiField label="项目上下文" v-slot="{ control }">
          <select v-bind="control" v-model="selectedId" :disabled="!!working || !!approval || loadingProjects">
            <option value="">选择项目（可选）</option>
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
            <p class="ai-bubble">我可以匹配软件产品、准备或发送询价，也可以查询项目进展。需要确认时，我会把内容交给你审批。</p>
            <div v-if="!turns.length" class="ai-prompts">
              <UiButton v-if="demo" compact icon="ai" :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch(sample)">试试 ERP 询价示例</UiButton>
              <UiButton v-else compact :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch('根据当前项目的采购需求准备询价草稿')">整理当前项目需求</UiButton>
              <UiButton compact :disabled="loadingProjects || !!working || !!pendingWrite" @click="dispatch('帮我查一下当前项目进展')">查看项目进展</UiButton>
            </div>
          </div>
        </div>
        <div v-for="turn in turns" :key="turn.id" class="ai-turn" :class="{ 'is-user': turn.role === 'user' }">
          <div v-if="turn.role === 'assistant'" class="ai-avatar"><UiIcon name="ai" :size="19" /></div>
          <div class="ai-turn-content">
            <p class="ai-speaker">{{ turn.role === 'user' ? '你' : '渠道业务助手' }}</p>
            <p class="ai-bubble">{{ turn.text }}</p>
            <UiButton v-if="turn.manualSource" compact :disabled="!!working || !!approval || !!pendingWrite" @click="fillManually(turn.manualSource)">手工填写需求</UiButton>

            <div v-if="turn.approval" class="ai-result">
              <div class="ai-result-head">
                <strong>{{ turn.approval.title }}</strong>
                <UiBadge :tone="approval === turn.approval ? 'warning' : 'neutral'">{{ approval === turn.approval ? '待人工审批' : '审批已结束' }}</UiBadge>
              </div>
              <UiButton v-if="approval === turn.approval" compact icon="permission" @click="reopenApproval">打开审批</UiButton>
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
                <UiButton v-if="choices && choices === turn.choices" compact variant="primary" :disabled="!!working || !!approval || !!pendingWrite"
                  @click="useCandidate(turn.choices, item.product)">选用并保存草稿</UiButton>
              </article>
            </div>

            <article v-if="turn.draft" class="ai-result">
              <div class="ai-result-head"><strong>询价草稿 <template v-if="turn.product">· {{ turn.product.name }}</template></strong><UiBadge tone="success">已保存</UiBadge></div>
              <p>{{ turn.draft.summary }}</p>
              <div class="ai-prompts">
                <UiButton compact end-icon="open-detail" @click="emit('open-project', turn.draft.projectId)">打开项目</UiButton>
                <UiButton compact icon="permission" :disabled="!!working || !!approval || !!pendingWrite" @click="reviewSavedDraft(turn)">审批发送询价</UiButton>
              </div>
            </article>

            <article v-if="turn.project" class="ai-result">
              <div class="ai-result-head"><strong>{{ turn.project.title }}</strong><UiBadge tone="info">{{ turn.project.stageLabel }}</UiBadge></div>
              <p v-if="turn.project.nextStep">下一步：{{ turn.project.nextStep }}</p>
              <UiButton compact end-icon="open-detail" @click="emit('open-project', turn.project.id)">打开项目</UiButton>
            </article>
          </div>
        </div>
      </div>

      <div class="ai-composer">
        <div v-if="working" class="ai-working" role="status"><UiIcon name="loading" :size="16" /> {{ working }}…</div>
        <div v-if="pendingWrite && !working" class="ai-recovery" role="status">
          <span>上次提交结果待确认。{{ recoveryError }}</span>
          <UiButton compact :disabled="!!working" @click="recoverWrite">用原请求确认结果</UiButton>
        </div>
        <form class="ai-composer-form" @submit.prevent="dispatch()">
          <textarea v-model="input" aria-label="给渠道业务助手发消息" rows="2" maxlength="2000" placeholder="例如：帮我整理这个项目的采购需求并准备询价草稿"
            :disabled="!!pendingWrite" @keydown.enter="onEnter" />
          <UiButton type="submit" variant="primary" end-icon="forward" :disabled="!input.trim() || !!working || loadingProjects || !!pendingWrite">发送</UiButton>
        </form>
        <p class="ai-composer-hint">Enter 发送 · Shift + Enter 换行</p>
      </div>
    </div>

    <ElDialog :model-value="approvalOpen" :title="approval?.title || '人工审批'" width="min(94vw, 660px)"
      :close-on-click-modal="false" @update:model-value="value => { if (!value) closeApproval() }">
      <form v-if="approval" class="ai-approval" @submit.prevent="approve">
        <template v-if="approval.kind === 'requirement'">
          <div class="ai-result-head"><strong>{{ workspaceSkills[approval.intent] }}</strong><UiBadge tone="warning">待人工审批</UiBadge></div>
          <p>{{ approval.intent === 'match_products' ? '同意后自动查询产品和匹配依据。' : '同意后自动匹配产品，候选明确时保存询价草稿。' }}</p>
          <details><summary>查看需求原文</summary><p class="ai-original">{{ approval.source }}</p></details>
          <UiField v-if="approval.intent !== 'match_products'" label="关联项目" v-slot="{ control }">
            <select v-bind="control" v-model="approval.projectId" :required="!demo">
              <option value="">{{ demo ? '保存时新建本地演示项目' : '选择待询价项目' }}</option>
              <option v-for="project in inquiryProjects" :key="project.id" :value="project.id">{{ project.title }} · {{ project.id }}</option>
            </select>
          </UiField>
          <UiButton v-if="!demo && approval.intent !== 'match_products' && !inquiryProjects.length" compact variant="text" @click="emit('open-projects')">前往项目协作建项</UiButton>
          <div class="ai-fields">
            <UiField v-for="field in fields" :key="field.key" :label="field.label" :hint="field.hint" v-slot="{ control }">
              <input v-if="field.numeric" v-bind="control" v-model="approval.fields[field.key]" type="number" min="1" max="1000000" placeholder="待确认">
              <input v-else v-bind="control" v-model="approval.fields[field.key]" :maxlength="field.max" placeholder="待确认">
            </UiField>
          </div>
          <aside v-if="approval.questions.length" class="ai-follow-up">
            <strong><UiIcon name="info" :size="16" /> 建议核实</strong>
            <ul><li v-for="(question, index) in approval.questions" :key="index">{{ question }}</li></ul>
          </aside>
        </template>
        <template v-else>
          <div class="ai-result-head"><strong>发送厂商询价</strong><UiBadge tone="warning">待人工审批</UiBadge></div>
          <p>同意后，以下内容将提交给该产品所属厂商，项目会进入待报价阶段。</p>
          <dl class="ai-send-details">
            <div><dt>项目</dt><dd>{{ approval.project.title }}</dd></div>
            <div><dt>产品</dt><dd>{{ approval.product.name }} {{ approval.product.version }}</dd></div>
            <div v-if="approval.product.manufacturerName || approval.product.manufacturer_id"><dt>厂商</dt><dd>{{ approval.product.manufacturerName || '#' + approval.product.manufacturer_id }}</dd></div>
            <div><dt>询价摘要</dt><dd>{{ approval.summary }}</dd></div>
          </dl>
        </template>
        <p v-if="approvalError" class="form-error" role="alert">{{ approvalError }}</p>
        <div class="ai-approval-actions">
          <UiButton @click="declineApproval">不同意</UiButton>
          <UiButton type="submit" variant="primary" icon="check">{{ approval.kind === 'send' ? '同意发送询价' : '同意并继续' }}</UiButton>
        </div>
      </form>
    </ElDialog>
  </section>
</template>

<style scoped src="./ai-workspace.css"></style>
