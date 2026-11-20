<script setup>
import { computed, ref, watch } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiCard, UiMetricCard, UiState } from '../ui/index.js'
import OpportunityForm from './OpportunityForm.vue'
import { categoryLabels, contactFields, opportunityStages, systemLabels } from './opportunities.js'
import { statusTone } from '../ui/statusTone.js'

const props = defineProps({ desk:Object, repo:Object, projects:Array, selected:String, captureFor:String, refresh:Function, live:Boolean, locked:Boolean })
const emit = defineEmits(['select','capture-opened','project','notice'])
const tab = ref('ledger'), filter = ref(''), search = ref(''), pending = ref(false), error = ref('')
const form = ref(null), match = ref(null), startingRevision = ref(0), contextId = ref(''), returningFor = ref('')
const current = computed(() => props.desk.items.find(item => item.id === props.selected))
const sourceProject = computed(() => props.projects.find(entry => entry.id === current.value?.projectId))
const today = () => new Date().toLocaleDateString('sv-SE')
const outstanding = item => item.stage === 'introduced' || (item.stage === 'won' && Number(item.feeAmount) > 0)
const due = item => (item.stage === 'open' || outstanding(item)) && item.followUp && item.followUp <= today()
const stats = computed(() => ({
  open:props.desk.items.filter(item => item.stage === 'open').length,
  due:props.desk.items.filter(due).length,
  won:props.desk.items.filter(item => ['won','settled'].includes(item.stage)).length,
}))
const linksFor = id => props.desk.reverseMatches.filter(row => row.partnerId === id)
const rows = computed(() => props.desk.items.filter(item =>
  (!filter.value || (filter.value === 'due' ? due(item) : filter.value === 'successful' ? ['won','settled'].includes(item.stage) : item.stage === filter.value)) &&
  (!returningFor.value || linksFor(returningFor.value).some(link => link.opportunityId === item.id)) &&
  [item.id,item.note,item.projectTitle,categoryLabels[item.category]].join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))
const titles = {capture:'从拜访记录整理商机',partner:'登记伙伴能力',clarify:'澄清附加需求',introduce:'登记引荐',win:'登记合作结果',lose:'登记未促成',settle:'登记介绍费结清',follow:'调整跟进日期',contactReview:'联系前核对',rejectPartner:'记录未匹配原因'}
async function acknowledge() {
  if (pending.value || props.locked) return
  pending.value = true
  try { await props.repo.actOpportunity(current.value.id,'acknowledge',{},current.value.revision); await props.refresh() }
  catch (cause) { emit('notice',cause.message); if (cause.status === 409) await props.refresh() }
  finally { pending.value = false }
}
const activePartner = computed(() => props.desk.partners.find(entry => entry.id === current.value?.partnerId))
function open(mode, candidate = null, projectId = '') {
  if (pending.value || props.locked) return
  startingRevision.value = current.value?.revision; error.value = ''; match.value = candidate
  contextId.value = projectId; form.value = mode
}
watch(() => props.captureFor, id => { if (id) { open('capture',null,id); emit('capture-opened') } }, { immediate:true })
watch(() => props.selected, () => { form.value = null })
watch(() => props.locked, locked => { if (locked) form.value = null })
function revisit(id) {
  returningFor.value = id; tab.value = 'ledger'; filter.value = ''; search.value = ''
}
function choose(section, stage = '') { tab.value = section; filter.value = stage; search.value = ''; returningFor.value = '' }
async function save(payload) {
  if (pending.value || props.locked) return
  const mode = form.value
  pending.value = true; error.value = ''
  try {
    let record
    if (mode === 'capture') {
      const { projectId, ...input } = payload
      record = await props.repo.captureOpportunity(projectId,input)
    } else if (mode === 'partner') record = await props.repo.addPartner(payload)
    else record = await props.repo.actOpportunity(current.value.id,mode,payload,startingRevision.value)
    await props.refresh()
    form.value = null
    if (mode === 'partner') { revisit(record.id); emit('notice','已登记伙伴，请查看可回访的历史商机') }
    else { emit('select',record.id); emit('notice','商机记录已更新') }
  } catch (cause) {
    error.value = mode === 'introduce' && cause.code === 'invalid_request' && Object.values(match.value?.checks || {}).includes('blocked')
      ? '联系前核对仍有未解决的问题，请先更新核对结果再引荐。'
      : cause.message
    if (cause.code === 'CONFLICT' || cause.status === 409) {
      await props.refresh()
      form.value = null
      emit('notice',cause.message + ' 请重新打开操作核对最新记录。')
    }
  } finally { pending.value = false }
}
async function copyBrief() {
  try { await navigator.clipboard.writeText(current.value.brief); emit('notice','已复制询问摘要，可自行发送') }
  catch { emit('notice','可选中摘要文字手动复制') }
}
</script>

<template>
  <div class="op-desk">
    <template v-if="!selected">
      <header class="page-heading ch-head"><div><p class="ch-kicker">代理商团队 · {{ live ? '正式商机' : '本地演示' }}</p><h1>商机拾遗</h1><p class="muted">客户额外提的需求，团队里谁能接？从一条走访记录找到有合作依据的伙伴。</p></div>
        <UiButton variant="primary" icon="add" @click="open('capture')">记录附加需求</UiButton>
      </header>
      <section class="metric-grid" aria-label="商机概况">
        <UiMetricCard label="待承接需求" :value="stats.open" icon="request" interactive @click="choose('ledger', 'open')" />
        <UiMetricCard label="今天需跟进" :value="stats.due" description="含逾期" icon="time" interactive @click="choose('ledger', 'due')" />
        <UiMetricCard label="已促成合作" :value="stats.won" icon="check" interactive @click="choose('ledger', 'successful')" />
      </section>
      <UiCard v-if="desk.tasks?.length" title="今天从这里接着办">
        <div class="op-records"><button v-for="task in desk.tasks" :key="task.opportunityId" @click="$emit('select',task.opportunityId)"><span><b>{{ task.reason }}</b><small>{{ task.projectTitle }}</small></span><span>{{ task.action }}</span></button></div>
      </UiCard>
      <nav class="op-tabs" aria-label="商机视图"><button v-for="[key,label] in [['ledger','商机台账'],['partners','团队伙伴'],['gaps','能力缺口']]" :key="key" :aria-pressed="tab === key" @click="choose(key)">{{ label }}</button></nav>
      <UiCard v-if="tab === 'ledger'" title="商机台账">
        <div class="op-tools"><input v-model="search" aria-label="搜索商机" placeholder="搜索需求、项目或编号"><select v-model="filter" aria-label="商机阶段"><option value="">全部阶段</option><option v-for="(label,key) in opportunityStages" :key="key" :value="key">{{ label }}</option><option value="due">今天需跟进</option><option value="successful">全部已促成</option></select>
          <UiButton v-if="returningFor" compact @click="returningFor = ''">清除伙伴回访筛选</UiButton></div>
        <p v-if="returningFor" class="op-inset">可请「{{ desk.partners.find(p => p.id === returningFor)?.name }}」协助的待承接需求（{{ rows.length }}）</p>
        <div v-if="rows.length" class="op-records"><button v-for="item in rows" :key="item.id" @click="$emit('select',item.id)">
          <span><small>{{ item.projectTitle }} · {{ item.id }}</small><b>{{ item.note }}</b><small>{{ categoryLabels[item.category] || '需澄清承接类型' }}<template v-if="item.followUp"> · 下次跟进 {{ item.followUp }}</template></small></span>
          <UiBadge :tone="statusTone(item.stage)" dot>{{ opportunityStages[item.stage] }}</UiBadge>
        </button></div><p v-else class="op-muted">暂无符合条件的商机。</p>
      </UiCard>
      <UiCard v-else-if="tab === 'partners'" title="团队伙伴" description="记录合作能力与内部引荐人。先找同事核实，不公开私人通讯录。">
        <template #actions><UiButton compact icon="add" @click="open('partner')">登记伙伴</UiButton></template>
        <article v-for="partner in desk.partners" :key="partner.id" class="op-line"><h3>{{ partner.name }}</h3><p>{{ categoryLabels[partner.category] }} · 内部引荐人：{{ partner.owner }}</p>
          <p class="op-muted">{{ partner.evidence || '尚无经授权的合作案例。' }}</p><UiButton compact @click="revisit(partner.id)">可回访商机（{{ linksFor(partner.id).length }}）</UiButton></article>
      </UiCard>
      <UiCard v-else title="团队能力缺口" description="按待承接商机聚合。用于决定下次找哪类伙伴，不向厂商公开客户信息。">
        <p v-if="!desk.gaps.length" class="op-muted">暂无已分类的待承接需求。</p>
        <div v-for="gap in desk.gaps" :key="gap.category" class="op-count"><span>{{ gap.label }}</span><b>{{ gap.count }} 条</b></div>
      </UiCard>
    </template>
    <template v-else-if="current">
      <UiButton compact variant="text" icon="back" @click="$emit('select','')">返回商机台账</UiButton>
      <header class="ch-project-head"><p class="ch-kicker">附加需求 · {{ current.id }}</p><h1>商机跟进</h1><div class="ch-project-meta"><UiBadge :tone="statusTone(current.stage)" dot>{{ opportunityStages[current.stage] }}</UiBadge><button v-if="sourceProject" class="op-link" @click="$emit('project',sourceProject)">{{ current.projectTitle }} · 查看主单</button></div></header>
      <div class="op-columns"><div class="ch-stack">
        <UiCard title="客户还需要什么"><p class="op-note">{{ current.note }}</p><p>{{ categoryLabels[current.category] || '承接类型待澄清' }} · {{ [systemLabels[current.source],systemLabels[current.target]].filter(Boolean).join(' → ') || '系统待确认' }}</p>
          <ul v-if="current.questions.length"><li v-for="q in current.questions" :key="q">{{ q }}</li></ul>
          <template v-if="current.stage === 'open'" #footer><UiButton compact @click="open('clarify')">澄清需求</UiButton></template>
        </UiCard>
        <UiCard v-if="current.stage === 'open'" title="团队里谁做过" description="优先列出有合作记录的伙伴。具体版本、排期仍需核实。">
          <UiButton v-if="current.matches.length" compact :disabled="pending" @click="acknowledge">已查看这批人选</UiButton>
          <p v-if="!current.matches.some(m => m.evidence)" class="op-muted">{{ current.questions.length ? '先澄清需求，再查找有依据的人选。' : '暂无有案例佐证的人选；可在团队伙伴中登记新的能力。' }}</p>
          <article v-for="candidate in current.matches" :key="candidate.partnerId" class="op-line"><h3>{{ candidate.name }} <UiBadge>{{ candidate.evidence ? '有合作记录' : '仅能力登记' }}</UiBadge></h3>
            <p>找 {{ candidate.owner }} 引荐 · {{ candidate.reason }}</p><p>{{ candidate.evidence }}</p><p class="op-muted">{{ candidate.caveat }}</p>
            <dl class="ch-facts"><template v-for="(label,key) in contactFields" :key="key"><dt>{{ label }}</dt><dd><UiBadge :tone="candidate.checks?.[key] === 'blocked' ? 'danger' : candidate.checks?.[key] === 'confirmed' ? 'success' : 'warning'">{{ {confirmed:'已确认可行',blocked:'存在问题'}[candidate.checks?.[key]] || '待确认' }}</UiBadge></dd></template></dl>
            <p v-if="candidate.reviewNote">{{ candidate.reviewNote }}</p>
            <div class="op-tools"><UiButton compact @click="open('contactReview',candidate)">联系前核对</UiButton><UiButton compact @click="open('rejectPartner',candidate)">记录未匹配</UiButton><UiButton compact @click="open('introduce',candidate)">登记引荐</UiButton></div></article>
        </UiCard>
        <UiCard v-if="current.rejections?.length" title="已排除的伙伴"><article v-for="entry in current.rejections" :key="entry.partnerId" class="op-line"><h3>{{ entry.name }}</h3><p>{{ entry.reason }}</p></article></UiCard>
        <UiCard v-if="activePartner" title="引荐与介绍费"><dl class="ch-facts"><dt>伙伴 / 引荐人</dt><dd>{{ activePartner.name }} / {{ activePartner.owner }}</dd><dt>跟进日期</dt><dd>{{ current.followUp }}</dd><dt>约定金额</dt><dd>{{ current.feeAmount ? '¥' + current.feeAmount : '未约定' }}</dd><dt>条件</dt><dd>{{ current.feeTerms || '未约定' }}</dd></dl></UiCard>
        <UiCard v-if="current.outcome" title="跟进结果"><p>{{ current.outcome }}</p></UiCard>
        <div class="op-tools"><UiButton v-if="current.stage === 'introduced'" variant="primary" @click="open('win')">登记已促成</UiButton>
          <UiButton v-if="current.stage === 'open' || outstanding(current)" @click="open('follow')">约定跟进日期</UiButton>
          <UiButton v-if="['open','introduced'].includes(current.stage)" @click="open('lose')">登记未促成</UiButton>
          <UiButton v-if="current.stage === 'won' && Number(current.feeAmount)>0" @click="open('settle')">登记介绍费结清</UiButton></div>
      </div><div class="ch-stack">
        <UiCard title="向伙伴询问的摘要" description="仅含已选能力和系统类型，可复制到微信询问。"><p class="op-inset">{{ current.brief }}</p><UiButton compact @click="copyBrief">复制询问摘要</UiButton></UiCard>
        <UiCard title="团队跟进记录"><ol class="ch-timeline"><li v-for="(event,index) in [...current.history].reverse()" :key="index"><time>{{ new Date(event.at).toLocaleDateString('zh-CN') }}</time><strong>{{ event.label }}</strong></li></ol></UiCard>
      </div></div>
    </template>
    <UiState v-else title="这条商机不存在或不可见"><UiButton @click="$emit('select','')">返回商机台账</UiButton></UiState>
    <ElDialog :model-value="!!form" :title="titles[form]" width="min(94vw, 600px)" :close-on-click-modal="false" :show-close="!pending" @update:model-value="value => { if (!value && !pending) form=null }">
      <OpportunityForm v-if="form" :key="form" :mode="form" :item="current" :partner="match" :project-id="contextId" :projects="projects" :busy="pending" :error="error" @save="save" @cancel="form=null" />
    </ElDialog>
  </div>
</template>
