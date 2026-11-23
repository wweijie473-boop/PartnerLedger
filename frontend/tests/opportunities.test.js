import test from 'node:test'
import assert from 'node:assert/strict'
import { actOnProject, demoActor, seedState, viewProject } from '../src/channel/domain.js'
import { actOpportunity, addPartner, captureOpportunity, opportunityDesk, previewPartnerMatches, suggestOpportunity } from '../src/channel/opportunities.js'
import { createDemoRepository, demoStorageKey } from '../src/channel/repository.js'
import { createHttpOpportunityRepository } from '../src/channel/httpOpportunities.js'

const now = new Date('2026-11-20T08:30:00Z'), dealer = demoActor('reseller'), vendor = demoActor('manufacturer')
const scope = {category:'integration',source:'erp',target:'wms'}
const note = '测试客户内部名称 预算十万 电话隐私'
function fixture() {
  const state = seedState(now), project = state.projects[0]
  const capture = payload => captureOpportunity(state,dealer,project.id,payload,now)
  const act = (item,action,payload) => actOpportunity(state,dealer,item.id,action,payload,item.revision,now)
  const desk = () => opportunityDesk(state,dealer,now)
  return {state,project,capture,act,desk}
}
test('拜访备注可在保存前推荐伙伴，确认后的商机保留相同的匹配条件', () => {
  const { capture, desk } = fixture()
  const { category, source, target, suggestedNote } = suggestOpportunity('ERP 已定，客户还想把 ERP 工单同步到 MES。')
  const criteria = { category, source, target }
  assert.deepEqual(criteria,{category:'integration',source:'erp',target:'mes'})
  const { items, partners } = desk()
  const preview = previewPartnerMatches(criteria,partners)
  assert.ok(preview.matches.some(candidate => candidate.owner === '王经理'))
  assert.match(preview.brief,/ERP → MES/)
  assert.equal(desk().items.length,items.length)
  const saved = capture({note:suggestedNote,...criteria})
  assert.deepEqual(saved.matches,preview.matches)
  const unclear = previewPartnerMatches(suggestOpportunity('客户还需要 IDE 接口'),partners)
  assert.ok(unclear.questions.length)
  assert.deepEqual(unclear.matches,[])
})
test('模糊接口不自动推断，未关联的私密字段不进入询问摘要或其他角色视图', () => {
  const {state,project,capture,act} = fixture()
  const draft = suggestOpportunity('ERP 已定，客户还需要 IDE 接口，原厂没有')
  assert.equal(draft.category,''); assert.equal(draft.suggestedNote,'客户还需要 IDE 接口')
  const initial = capture({note})
  assert.ok(initial.questions.length); assert.deepEqual(initial.matches,[])
  const result = act(initial,'clarify',scope)
  assert.match(result.brief,/ERP → WMS/); assert.doesNotMatch(result.brief,/电话|预算|内部名称|PRJ-|GAP-/)
  assert.equal(project.revision,1)
  const other = {...dealer, orgId:'unrelated'}
  for (const actor of [vendor,demoActor('customer')]) assert.throws(() => opportunityDesk(state,actor,now), {code:'FORBIDDEN'})
  assert.deepEqual(opportunityDesk(state,other,now).items,[])
  assert.throws(() => captureOpportunity(state,other,project.id,{note},now),{code:'NOT_FOUND'})
  assert.throws(() => actOpportunity(state,other,initial.id,'lose',{outcome:'x'},1,now),{code:'NOT_FOUND'})
  assert.doesNotMatch(JSON.stringify(viewProject(project,demoActor('customer'),now)),/GAP-|私密字段|测试客户内部名称/)
})
test('厂商记录能力缺口，主询价仍可报价，客户看不到上游说明', () => {
  const {state} = fixture(), project = state.projects.find(item => item.stage === 'inquired'), oldRevision = project.revision
  const result = actOnProject(state,vendor,project.id,'flagGap',{note:'本厂不支持特定采集插件'},oldRevision,now)
  assert.equal(result.stage,'inquired'); assert.ok(result.actions.includes('channelQuote')); assert.ok(!result.actions.includes('flagGap'))
  assert.equal(viewProject(project,dealer,now).inquiry.gapNote,'本厂不支持特定采集插件')
  assert.doesNotMatch(JSON.stringify(viewProject(project,demoActor('customer'),now)),/特定采集插件/)
  assert.equal(opportunityDesk(state,dealer,now).items.filter(item => item.projectId === project.id).length,0)
})
test('新伙伴反向唤回旧需求；双向同意、版本、金额和结清顺序受校验', () => {
  const {state,capture,act,desk} = fixture()
  const item = capture({note,...scope})
  assert.equal(item.matches.length,0)
  const manual = addPartner(state,dealer,{name:'未核实团队',owner:'同事甲',...scope},now)
  const verified = addPartner(state,dealer,{name:'交付团队',owner:'同事乙',evidence:'已验收项目凭据',...scope},now)
  const own = () => desk().items.find(entry => entry.id === item.id)
  assert.equal(own().matches[0].partnerId,verified.id)
  assert.match(own().matches.find(match => match.partnerId === manual.id).caveat,/无案例/)
  assert.ok(desk().reverseMatches.some(link => link.partnerId === verified.id && link.opportunityId === item.id))
  const input = {partnerId:verified.id,feeAmount:'1000.5',followUp:'2026-11-23',feeTerms:'验收后',consent:true}
  assert.throws(() => act(item,'introduce',{...input,consent:false}))
  assert.throws(() => act(item,'introduce',{...input,feeTerms:''}))
  assert.throws(() => act(item,'introduce',{...input,feeAmount:'-2'}))
  const introduced = act(item,'introduce',input)
  assert.equal(introduced.feeAmount,'1000.50'); assert.equal(introduced.partnerId,verified.id)
  assert.throws(() => act(item,'introduce',input),{code:'CONFLICT'})
  assert.throws(() => act(introduced,'settle',{}),{code:'CONFLICT'})
  const later = act(introduced,'follow',{followUp:'2026-12-01'})
  const won = act(later,'win',{outcome:'已签约 客户真实联系人',remember:true})
  const evidence = desk().partners.find(entry => entry.id === verified.id).evidence
  assert.match(evidence,new RegExp(item.id)); assert.doesNotMatch(evidence,/真实联系人/)
  const settled = act(won,'settle',{})
  assert.equal(settled.stage,'settled')
  assert.ok(!desk().reverseMatches.some(link => link.opportunityId === item.id))
})
test('旧演示数据保留，商机和伙伴可跨页面读取；重置同时恢复示例', async () => {
  const old = seedState(now)
  old.projects[0].title = '旧台账保留'
  let bytes = JSON.stringify(old)
  const storage = {getItem:key => key === demoStorageKey ? bytes : null,setItem:(_,value) => {bytes = value}}
  const repo = () => createDemoRepository('reseller',{storage,now:() => now})
  const initial = await repo().opportunityDesk()
  assert.ok(initial.items.length); assert.equal((await repo().list())[0].title,'旧台账保留')
  await repo().captureOpportunity(old.projects[0].id,{note,...scope})
  assert.equal((await repo().opportunityDesk()).items.length,initial.items.length + 1)
  await repo().reset(); assert.equal((await repo().opportunityDesk()).items.length,initial.items.length)
})
test('商机 HTTP 适配器保留版本与幂等键，不在失败时写入假记录', async () => {
  const calls = [], failure = Object.assign(new Error('版本冲突'),{status:409})
  const repo = createHttpOpportunityRepository(async (url,options) => { calls.push({url,options}); if (options) throw failure; return {items:[]} })
  assert.deepEqual(await repo.opportunityDesk(),{items:[]})
  await assert.rejects(repo.actOpportunity('A/B','win',{outcome:'完成'},4,'same-id'),{status:409,requestId:'same-id'})
  assert.match(calls[1].url,/A%2FB\/actions\/win$/)
  assert.deepEqual(calls[1].options.body,{payload:{outcome:'完成'},revision:4,requestId:'same-id'})
})
