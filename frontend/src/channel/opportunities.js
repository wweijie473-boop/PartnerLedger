import { roles } from './domain.js'

export const categoryLabels = Object.freeze({
  integration: '系统对接服务', connector: '成品接口 / 连接器', plugin: '开发工具插件',
  migration: '数据迁移', training: '实施培训', other: '其他能力',
})
export const systemLabels = Object.freeze({
  erp: 'ERP', mes: 'MES', oa: 'OA', ide: 'IDE', wms: 'WMS', crm: 'CRM', other: '其他系统',
})
export const opportunityStages = Object.freeze({
  open: '待跟进', introduced: '已引荐', won: '已促成', lost: '未促成', settled: '介绍费已结清',
})
export const contactFields = Object.freeze({
  version: '软件与接口版本', interfaceAccess: '接口开放条件', region: '实施地区与方式', availability: '交付档期',
})
const twoSystems = ['integration', 'connector', 'migration']
const copy = value => structuredClone(value)
function reject(message, code = 'INVALID') { throw Object.assign(new Error(message), { code }) }
function guard(actor) {
  if (actor?.role !== 'reseller' || !actor.orgId) reject('仅经办代理商可使用商机拾遗。', 'FORBIDDEN')
}
function projectFor(state, actor, id) {
  const project = state.projects.find(entry => entry.id === id && entry.resellerId === actor.orgId)
  if (!project) reject('关联项目不存在或不可见。', 'NOT_FOUND')
  return project
}
function textField(value, label, limit, required = true) {
  if (value == null && !required) return ''
  if (typeof value !== 'string' || value.trim().length > limit || (required && !value.trim())) reject('请检查' + label + '。')
  return value.trim()
}
function choice(dictionary, value, required = false) {
  if ((!value && !required) || Object.hasOwn(dictionary, value)) return value || ''
  reject('请选择有效的能力类型或系统。')
}
function scope(payload, requiredCategory) {
  return {
    category: choice(categoryLabels, payload.category, requiredCategory),
    source: choice(systemLabels, payload.source), target: choice(systemLabels, payload.target),
  }
}
function questionsFor({ category, source, target }) {
  if (!category) return ['需要购买成品接口、委托开发对接服务，还是使用开发工具插件？']
  if (category === 'other') return ['这项能力暂未归类，请先与客户澄清；不会自动推荐人选。']
  const questions = []
  if (!source || source === 'other') questions.push('涉及哪类软件或开发工具？')
  if (twoSystems.includes(category) && (!target || target === 'other')) questions.push('数据要从哪个系统连接或迁移到哪个目标系统？')
  return questions
}
export function suggestOpportunity(note = '') {
  const residual = note.split(/[；;。\n，,]/).map(part => part.trim()).filter(part => part && /另外|还需|还要|额外|另需|还想|也需要/.test(part))
  const suggestedNote = residual.join('；') || note.trim()
  const options = [
    ['plugin', /插件/], ['migration', /迁移/], ['training', /培训/],
    ['integration', /系统对接|对接服务|定制开发/], ['connector', /成品接口|连接器/],
  ]
  const found = options.filter(([, rule]) => rule.test(suggestedNote))
  const pair = suggestedNote.match(/\b(ERP|MES|OA|IDE|WMS|CRM)\b\s*[\u4e00-\u9fff]{0,12}?(对接|连接|同步|→|至|到)\s*[\u4e00-\u9fff]{0,8}?\s*\b(ERP|MES|OA|IDE|WMS|CRM)\b/i)
  const category = found.length === 1 ? found[0][0] :
    !found.length && pair && /对接|连接|同步/.test(pair[2]) ? 'integration' : ''
  const single = suggestedNote.match(/\b(ERP|MES|OA|IDE|WMS|CRM)\b\s*(?:插件|培训)/i)
  const result = { category, source: pair?.[1].toLowerCase() || single?.[1].toLowerCase() || '', target: pair?.[3].toLowerCase() || '' }
  return { ...result, suggestedNote, questions: questionsFor(result) }
}
function safeBrief(item) {
  const systems = [systemLabels[item.source], systemLabels[item.target]].filter(Boolean)
  return ['有一项企业软件配套需求。', '能力：' + (categoryLabels[item.category] || '待澄清') + '。',
    ...(systems.length ? ['涉及系统：' + systems.join(' → ') + '。'] : []),
    '请确认支持的版本、交付方式和是否可承接。'].join('')
}
function matchesFor(item, partners) {
  if (questionsFor(item).length) return []
  return partners.filter(partner => {
    if (item.rejections?.some(entry => entry.partnerId === partner.id)) return false
    if (partner.category !== item.category) return false
    return ['source', 'target'].every(key => !item[key] || !partner[key] || item[key] === partner[key])
  }).map(partner => {
    const aligned = ['source', 'target'].filter(key => item[key] && item[key] === partner[key])
    const missing = ['source', 'target'].filter(key => item[key] && !partner[key])
    return { partnerId:partner.id, name:partner.name, owner:partner.owner, evidence:partner.evidence,
      checks:copy(item.contactReviews?.[partner.id]?.checks || {}),
      reviewNote:item.contactReviews?.[partner.id]?.note || '',
      reason: [categoryLabels[item.category], ...aligned.map(key => systemLabels[item[key]])].join(' · '),
      caveat: [!partner.evidence ? '仅有团队登记的能力，尚无案例佐证。' : '',
        missing.length ? '未记录对 ' + missing.map(key => systemLabels[item[key]]).join('、') + ' 的支持。' : '',
        '确切版本、档期和报价仍需向伙伴核实。'].join(''), specificity:aligned.length }
  }).sort((left, right) => Number(!!right.evidence) - Number(!!left.evidence) ||
    right.specificity - left.specificity || left.partnerId.localeCompare(right.partnerId))
}

export function previewPartnerMatches(capability, partners) {
  return { questions:questionsFor(capability), matches:matchesFor(capability, partners), brief:safeBrief(capability) }
}
function initialItem(id, orgId, projectId, note, spec, at) {
  return { id, orgId, projectId, note, ...spec, revision:1, stage:'open', partnerId:'', followUp:'',
    feeAmount:'', feeTerms:'', outcome:'', history:[{at,label:'确认记录附加需求'}] }
}
// Additive migration for existing three-role demo stores. No projects are recreated.
export function ensureOpportunityMemory(state, now) {
  if (state.opportunityMemory) {
    for (const item of state.opportunityMemory.items) {
      if (!item.knownPartnerIds) item.knownPartnerIds = matchesFor(item,state.opportunityMemory.partners.filter(partner => partner.orgId === item.orgId)).map(match => match.partnerId)
    }
    return state.opportunityMemory
  }
  const orgId = roles.reseller.orgId
  const memory = { counter:10, items:[], partners:[
    { id:'PPL-1',orgId,name:'桥川系统集成团队',owner:'王经理',category:'integration',source:'erp',target:'mes',evidence:'去年的制造项目已验收 ERP → MES 工单同步（团队示例记录）。' },
    { id:'PPL-2',orgId,name:'北岸实施工作室',owner:'李顾问',category:'training',source:'erp',target:'',evidence:'在财务共享项目完成 ERP 上线培训（团队示例记录）。' },
  ] }
  const sample = state.projects.find(project => project.id === 'PRJ-1021' && project.resellerId === orgId)
  if (sample) memory.items.push(initialItem('GAP-1', orgId, sample.id,
    'ERP 主单已经实施，客户还要把工单同步到车间 MES；原厂未提供对接服务。',
    { category:'integration',source:'erp',target:'mes' }, now.toISOString()))
  for (const item of memory.items) item.knownPartnerIds = matchesFor(item,memory.partners).map(match => match.partnerId)
  state.opportunityMemory = memory
  return memory
}
function projectItem(state, item, partners) {
  const { orgId, ...record } = item
  return { ...copy(record), projectTitle:state.projects.find(project => project.id === item.projectId)?.title ?? '',
    questions:questionsFor(item), matches:matchesFor(item, partners), brief:safeBrief(item) }
}
function workItems(items, now) {
  const today = [now.getFullYear(), String(now.getMonth()+1).padStart(2,'0'), String(now.getDate()).padStart(2,'0')].join('-')
  return items.flatMap(item => {
    const task = (reason,action) => ({opportunityId:item.id,projectTitle:item.projectTitle,reason,action,date:item.followUp || ''})
    if (item.stage === 'won' && Number(item.feeAmount) > 0) {
      return [task('合作已促成，需核对介绍费条件' + (item.feeTerms ? '（' + item.feeTerms + '）' : ''),'核对结清条件')]
    }
    if (!['open','introduced'].includes(item.stage)) return []
    if (item.followUp && item.followUp <= today) {
      return [task((item.followReason === 'customer' ? '客户约定答复' : '伙伴回访') + '已到日期 ' + item.followUp,'记录跟进结果')]
    }
    if (item.stage === 'open') {
      const known = item.knownPartnerIds || item.matches.map(match => match.partnerId)
      const added = item.matches.filter(match => !known.includes(match.partnerId))
      if (added.length) return [task('新匹配到 ' + added.map(match => match.name).join('、') + '，可以回访此需求','查看新伙伴依据')]
      if (item.questions.length) return [task('需要确认' + (categoryLabels[item.category] || '承接类型与涉及系统'),'补充需求信息')]
    }
    return []
  }).sort((left,right) => (left.date || '9999').localeCompare(right.date || '9999'))
}
function partnersFor(memory, actor) { return memory.partners.filter(partner => partner.orgId === actor.orgId) }
export function opportunityDesk(state, actor, now) {
  guard(actor)
  const memory = ensureOpportunityMemory(state, now)
  const partners = partnersFor(memory, actor)
  const visibleProjects = new Set(state.projects.filter(project => project.resellerId === actor.orgId).map(project => project.id))
  const items = memory.items.filter(item => item.orgId === actor.orgId && visibleProjects.has(item.projectId))
    .map(item => projectItem(state, item, partners))
  const active = items.filter(item => item.stage === 'open')
  const gaps = Object.entries(categoryLabels).map(([category,label]) =>
    ({category,label,count:active.filter(item => item.category === category).length})).filter(gap => gap.count).sort((a,b) => b.count-a.count)
  const reverseMatches = active.flatMap(item => item.matches.map(match => ({partnerId:match.partnerId,opportunityId:item.id})))
  return { items, partners:partners.map(({orgId,...partner}) => copy(partner)), gaps, reverseMatches, tasks:workItems(items,now) }
}
export function captureOpportunity(state, actor, projectId, payload, now) {
  guard(actor)
  projectFor(state, actor, projectId)
  const note = textField(payload.note, '拜访记录（1–2000字）', 2000)
  const spec = scope(payload, false)
  const memory = ensureOpportunityMemory(state, now)
  if (memory.items.some(item => item.projectId === projectId && item.note === note && item.orgId === actor.orgId))
    reject('同项目已有这条拜访记录。', 'CONFLICT')
  const item = initialItem('GAP-' + ++memory.counter, actor.orgId, projectId, note, spec, now.toISOString())
  item.knownPartnerIds = matchesFor(item,partnersFor(memory,actor)).map(match => match.partnerId)
  memory.items.unshift(item)
  return projectItem(state, item, partnersFor(memory, actor))
}
export function addPartner(state, actor, payload, now) {
  guard(actor)
  const details = { ...scope(payload, true), name:textField(payload.name,'伙伴名称',80),
    owner:textField(payload.owner,'内部引荐人',60), evidence:textField(payload.evidence,'合作依据',500,false) }
  const memory = ensureOpportunityMemory(state, now)
  const partner = {id:'PPL-' + ++memory.counter, ...details,createdAt:now.toISOString()}
  memory.partners.unshift({ ...partner,orgId:actor.orgId })
  return copy(partner)
}
function dateField(value) {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\d$/.test(value)) reject('请选择有效的跟进日期。')
  const parsed = new Date(value + 'T12:00:00Z')
  if (!Number.isFinite(parsed.valueOf()) || parsed.toISOString().slice(0,10) !== value) reject('跟进日期不存在。')
  return value
}
function amountField(value) {
  if (value === undefined || value === null || value === '') return ''
  const parts = /^(\d{1,8})(?:\.(\d{1,2}))?$/.exec(String(value))
  if (!parts) reject('介绍费金额应为非负数，最多两位小数。')
  return Number(parts[1]) + '.' + (parts[2] ?? '').padEnd(2,'0')
}
export function actOpportunity(state, actor, id, action, payload, revision, now) {
  guard(actor)
  const memory = ensureOpportunityMemory(state, now)
  const item = memory.items.find(entry => entry.id === id && entry.orgId === actor.orgId)
  if (!item) reject('商机不存在或不可见。', 'NOT_FOUND')
  projectFor(state, actor, item.projectId)
  if (item.revision !== revision) reject('商机记录已更新，请重新打开后再操作。', 'CONFLICT')
  const partners = partnersFor(memory, actor)
  let changes, label
  if (action === 'acknowledge' && item.stage === 'open') {
    changes = {knownPartnerIds:matchesFor(item,partners).map(match => match.partnerId)}
    label = '已查看当前伙伴依据'
  } else if (action === 'contactReview' && item.stage === 'open') {
    if (!matchesFor(item,partners).some(match => match.partnerId === payload.partnerId)) reject('当前伙伴已不可匹配。','CONFLICT')
    const checks = {}
    for (const key of Object.keys(contactFields)) {
      const value = payload.checks?.[key] || 'unknown'
      if (!['unknown','confirmed','blocked'].includes(value)) reject('请核对联系清单。')
      checks[key] = value
    }
    const detail = textField(payload.note,'联系备注',500,false)
    changes = {contactReviews:{...item.contactReviews,[payload.partnerId]:{checks,note:detail}}}
    label = '记录伙伴联系前核对结果'
  } else if (action === 'rejectPartner' && item.stage === 'open') {
    const match = matchesFor(item,partners).find(entry => entry.partnerId === payload.partnerId)
    if (!match) reject('当前伙伴已不可匹配。','CONFLICT')
    const reason = textField(payload.reason,'未匹配成的原因',500)
    changes = {rejections:[...(item.rejections || []),{partnerId:match.partnerId,name:match.name,reason,at:now.toISOString()}]}
    label = '记录与' + match.name + '未匹配成的原因'
  } else if (action === 'clarify' && item.stage === 'open') {
    changes = scope(payload, true); label = '更新能力与系统范围'
  } else if (action === 'introduce' && item.stage === 'open') {
    const match = matchesFor(item, partners).find(candidate => candidate.partnerId === payload.partnerId)
    if (!match || payload.consent !== true) reject('请核实伙伴能力并确认双方同意引荐。')
    if (Object.values(match.checks).includes('blocked')) reject('联系清单存在未解决的问题，请先核实并更新。')
    const feeAmount = amountField(payload.feeAmount)
    const feeTerms = textField(payload.feeTerms, '介绍费条件', 300, Number(feeAmount) > 0)
    changes = { stage:'introduced', partnerId:match.partnerId, followUp:dateField(payload.followUp),followReason:'partner',feeAmount,feeTerms }
    label = '经双方同意，由' + match.owner + '引荐至' + match.name
  } else if (action === 'follow' && (['open','introduced'].includes(item.stage) || (item.stage === 'won' && Number(item.feeAmount) > 0))) {
    const followReason = payload.followReason || (item.stage === 'open' ? 'customer' : item.stage === 'won' ? 'fee' : 'partner')
    if (!['customer','partner','fee'].includes(followReason)) reject('请选择跟进事项。')
    changes = {followUp:dateField(payload.followUp),followReason}
    label = '更新跟进日期至 ' + changes.followUp
  } else if ((action === 'win' && item.stage === 'introduced') || (action === 'lose' && ['open','introduced'].includes(item.stage))) {
    changes = { stage:action === 'win' ? 'won' : 'lost', outcome:textField(payload.outcome,'跟进结果',500) }
    label = action === 'win' ? '登记已促成合作' : '登记未促成原因'
    if (action === 'win' && payload.remember === true) {
      const partner = partners.find(entry => entry.id === item.partnerId)
      if (partner) {
        const evidence = '团队引荐 ' + item.id + ' 已促成' + categoryLabels[item.category] + '合作（成交记录，非交付验收）。'
        partner.evidence = [partner.evidence, evidence].filter(Boolean).join('\n')
      }
    }
  } else if (action === 'settle' && item.stage === 'won' && Number(item.feeAmount) > 0) {
    changes = {stage:'settled'}; label = '人工登记介绍费已结清'
  } else reject('当前阶段不允许此操作。','CONFLICT')
  Object.assign(item, changes)
  item.revision += 1
  item.history.push({at:now.toISOString(),label})
  return projectItem(state, item, partners)
}
