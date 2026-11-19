import test from 'node:test'
import assert from 'node:assert/strict'
import { demoActor, seedState, viewProject } from '../src/channel/domain.js'
import { actOpportunity, addPartner, captureOpportunity, contactFields, opportunityDesk } from '../src/channel/opportunities.js'
import { createVisitDraftStore, splitVisit } from '../src/live/visitDrafts.js'

const dealer = demoActor('reseller')
const now = new Date('2026-11-19T08:00:00Z')
const scope = {category:'integration',source:'erp',target:'wms'}
function fixture() {
  const state = seedState(now)
  const project = state.projects[0]
  const opportunity = captureOpportunity(state,dealer,project.id,{note:'客户还需要仓储同步',...scope},now)
  return {
    state,project,opportunity,
    desk:() => opportunityDesk(state,dealer,now),
    act:(action,payload) => {
      const current = opportunityDesk(state,dealer,now).items.find(item => item.id === opportunity.id)
      return actOpportunity(state,dealer,current.id,action,payload,current.revision,now)
    },
  }
}
test('新伙伴提醒有来源，查看后消除，新需求不改动主项目和其他端', () => {
  const {state,project,opportunity,desk,act} = fixture()
  const before = structuredClone(project)
  const partner = addPartner(state,dealer,{...scope,name:'仓储对接团队',owner:'顾问甲',evidence:'授权示例案例'},now)
  assert.ok(desk().tasks.some(task => task.opportunityId === opportunity.id && task.reason.includes(partner.name)))
  act('acknowledge',{})
  assert.ok(!desk().tasks.some(task => task.opportunityId === opportunity.id))
  assert.deepEqual(project,before)
  assert.doesNotMatch(JSON.stringify(viewProject(project,demoActor('customer'),now)),/仓储对接团队|顾问甲/)
})
test('联系清单存在问题时阻止引荐，排除原因保留且可以继续找其他人', () => {
  const {state,opportunity,desk,act} = fixture()
  const partner = addPartner(state,dealer,{...scope,name:'版本受限团队',owner:'顾问乙'},now)
  act('contactReview',{partnerId:partner.id,checks:{version:'blocked'},note:'只支持旧版'})
  assert.throws(() => act('introduce',{partnerId:partner.id,consent:true,followUp:'2026-11-20'}),/未解决/)
  const confirmed = Object.fromEntries(Object.keys(contactFields).map(key => [key,'confirmed']))
  act('contactReview',{partnerId:partner.id,checks:confirmed,note:'待确认客户预算，不可对外'})
  const current = desk().items.find(item => item.id === opportunity.id)
  assert.doesNotMatch(current.brief,/客户预算|顾问乙|旧版/)
  const rejected = act('rejectPartner',{partnerId:partner.id,reason:'本轮排期无法满足'})
  assert.equal(rejected.stage,'open')
  assert.equal(rejected.matches.length,0)
  assert.equal(rejected.rejections[0].reason,'本轮排期无法满足')
})
test('到期答复与合作费条件形成可办理待办，关闭后不再提醒', () => {
  const {opportunity,desk,act} = fixture()
  act('follow',{followUp:'2026-11-19',followReason:'customer'})
  assert.ok(desk().tasks.some(task => task.opportunityId === opportunity.id && task.reason.includes('客户约定答复')))
  act('lose',{outcome:'客户暂缓'})
  assert.ok(!desk().tasks.some(task => task.opportunityId === opportunity.id))
})
test('手机草稿刷新后可继续编辑，正式与演示以及不同用户相互隔离', () => {
  const memory = new Map()
  const storage = {getItem:key => memory.get(key),setItem:(key,value) => memory.set(key,value),removeItem:key => memory.delete(key)}
  const own = createVisitDraftStore('live:org:member',storage)
  const fields = splitVisit('ERP 已定，客户还需要 IDE 接口')
  assert.equal(fields.main,'ERP 已定')
  assert.equal(fields.additional,'客户还需要 IDE 接口')
  assert.equal(fields.category,'')
  const saved = own.save({text:'拜访一句话',projectId:'42'},0)
  const restored = createVisitDraftStore('live:org:member',storage).list()[0]
  assert.equal(restored.id,saved.id)
  assert.equal(createVisitDraftStore('demo:org',storage).list().length,0)
  assert.equal(createVisitDraftStore('live:org:other',storage).list().length,0)
  own.save({...restored,text:'更新内容'},restored.revision)
  assert.throws(() => own.save(restored,restored.revision),/其他页面更新/)
  own.clear()
  assert.equal(own.list().length,0)
})
