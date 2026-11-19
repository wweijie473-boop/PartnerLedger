import test from 'node:test'
import assert from 'node:assert/strict'
import { demoActor, viewProject, projectList, createProject, actOnProject, seedState } from '../src/channel/domain.js'
import { createDemoRepository, demoStorageKey } from '../src/channel/repository.js'

const dealer = demoActor('reseller'), vendor = demoActor('manufacturer'), client = demoActor('customer')
const start = new Date(2026, 10, 19, 10, 0)
const quote = (price, validUntil = '2026-11-30') => ({
  validUntil, deliveryDays: 10, lines: [{ label: '25 席授权', quantity: 25, unitPrice: price }, { label: '培训', quantity: 1, unitPrice: 1500 }],
})
function scenario() {
  const state = seedState(start)
  const initial = createProject(state, client, { title: 'ERP · 海川精工二期', requirement: '采购预算等仅代理商可见' }, start)
  const raw = state.projects.find(p => p.id === initial.id)
  return {
    state, raw, view: (actor, now = start) => viewProject(raw, actor, now),
    act: (actor, action, payload = {}, version = raw.revision, now = start) => actOnProject(state, actor, raw.id, action, payload, version, now),
    inquire: () => actOnProject(state, dealer, raw.id, 'submitInquiry', { product: 'ERP-PRO', summary: '25席，私有部署' }, raw.revision, start),
  }
}

test('ERP 采购包含补资料、两套报价改版、单独下单、供货、实施、整改与验收', () => {
  const { raw, view, act, inquire } = scenario()
  assert.equal(view(dealer).isTodo, true)
  act(dealer, 'requestInfo', { note: '是否需要历史数据迁移？' })
  act(client, 'replyInfo', { note: '含三年记录' })
  inquire()
  const upstreamView = view(vendor)
  assert.equal(upstreamView.inquiry.summary, '25席，私有部署')
  assert.doesNotMatch(JSON.stringify(upstreamView), /海川精工|采购预算|三年记录/)
  act(vendor, 'requestInfo', { note: '请补充部署环境' })
  act(dealer, 'requestInfo', { note: '请确认机房系统' })
  act(client, 'replyInfo', { note: '内部机房' })
  act(dealer, 'replyInfo', { note: '可私有部署' })
  assert.doesNotMatch(JSON.stringify(view(vendor)), /内部机房/)
  assert.ok(!view(dealer).actions.includes('withdrawInquiry'))
  act(vendor, 'channelQuote', quote(1500.15))
  assert.equal(view(dealer).channelQuotes[0].total, 39003.75)
  act(dealer, 'reviseChannel', { note: '渠道申请调价' })
  act(vendor, 'channelQuote', quote(1399.18))
  act(dealer, 'salesQuote', quote(1699.19))
  act(client, 'reviseSales', { note: '一期只需要三周培训' })
  act(dealer, 'salesQuote', quote(1699))
  const buyer = view(client), maker = view(vendor)
  assert.equal(buyer.salesQuotes[0].status, 'superseded')
  assert.equal(view(dealer).channelQuotes[0].status, 'superseded')
  assert.doesNotMatch(JSON.stringify(buyer), /1399.18|渠道申请调价|CQ-|channelQuotes/)
  assert.doesNotMatch(JSON.stringify(maker), /1699|一期只需要三周培训|SQ-|salesQuotes/)
  assert.throws(() => act(client, 'confirmSales', { quoteId: buyer.salesQuotes[0].id }), {code:'CONFLICT'})
  const salesId = buyer.salesQuotes[1].id
  act(client, 'confirmSales', { quoteId: salesId })
  assert.ok(!view(client).order)
  const version = raw.revision
  const ordered = act(dealer, 'placeOrder', { quoteId: salesId })
  assert.deepEqual(act(dealer, 'placeOrder', { quoteId: salesId }, version), ordered)
  assert.equal(view(client).order.channelQuoteId, undefined)
  assert.equal(view(vendor).order.salesQuoteId, undefined)
  act(vendor, 'supply', { note: '授权可用' })
  act(dealer, 'implement', { note: '完成安装，请验收。' })
  act(client, 'rectify', { note: '请补录培训签到' })
  assert.equal(view(dealer).stage, 'rectification')
  act(dealer, 'implement', { note: '已补录' })
  act(client, 'accept')
  assert.equal(view(client).stage, 'completed')
  assert.equal(view(vendor).timeline.at(-1).label, '项目已验收')
  assert.equal(view(dealer).timeline.filter(event => event.label === '代理商确认下单').length, 1)
})

test('未处理询价可撤回，重发时不带上一次询价的私有摘要', () => {
  const { act, inquire, view } = scenario()
  inquire()
  act(dealer, 'withdrawInquiry')
  assert.throws(() => view(vendor), {code:'NOT_FOUND'})
  act(dealer, 'submitInquiry', { product: 'MES-STD', summary: '新版摘要' })
  assert.doesNotMatch(JSON.stringify(view(vendor)), /25席/)
})

test('组织和角色控制投影与写操作，陈旧版本不覆盖数据', () => {
  const { state, raw, act, view, inquire } = scenario()
  for (const actor of [client,dealer,vendor]) {
    const other = { ...actor, orgId: 'another-org' }
    assert.deepEqual(projectList(state, other, start), [])
    assert.throws(() => view(other), {code:'NOT_FOUND'})
    assert.throws(() => act(other, 'requestInfo', {note:'不可见'}), {code:'NOT_FOUND'})
  }
  const version = raw.revision
  inquire()
  assert.throws(() => act(dealer, 'withdrawInquiry', {}, version), {code:'CONFLICT'})
  assert.throws(() => act(client, 'channelQuote', quote(100)), {code:'CONFLICT'})
  assert.throws(() => act(vendor, 'channelQuote', quote(0.333)), /两位小数/)
  assert.throws(() => createProject(state, dealer, { title:'线下客户', customerName:'另一企业',requirement:'新需求' }, start), /已绑定/)
  const entry = createProject(state, dealer, { title:'线下需求代录', customerName:'海川精工',requirement:'新需求' }, start)
  assert.ok(projectList(state, client, start).some(p => p.id === entry.id))
})

test('过期的对客报价不能确认或下单；双方可发新版', () => {
  const { raw, act, inquire, view } = scenario()
  const tomorrow = new Date(2026,10,20,12)
  inquire()
  act(vendor, 'channelQuote', quote(1500))
  act(dealer, 'salesQuote', quote(1700, '2026-11-19'))
  const original = view(client).salesQuotes[0]
  assert.equal(view(client, tomorrow).salesQuotes[0].status, 'expired')
  assert.throws(() => act(client, 'confirmSales', {quoteId:original.id}, raw.revision, tomorrow), {code:'CONFLICT'})
  act(client, 'confirmSales', {quoteId:original.id})
  assert.throws(() => act(dealer, 'placeOrder', {quoteId:original.id}, raw.revision, tomorrow), {code:'CONFLICT'})
  const revised = act(dealer, 'salesQuote', quote(1695), raw.revision, tomorrow)
  assert.equal(revised.stage, 'sales_quoted')
  assert.throws(() => act(dealer, 'placeOrder', {quoteId:original.id}), {code:'CONFLICT'})
  const quoteId = revised.salesQuotes.at(-1).id
  act(client, 'confirmSales', {quoteId}, raw.revision, tomorrow)
  assert.equal(act(dealer, 'placeOrder', {quoteId},raw.revision,tomorrow).order.salesQuoteId,quoteId)
})

test('上游报价过期后也不能下单，重新出价将旧的销售确认作废', () => {
  const { raw, act, inquire, view } = scenario()
  const tomorrow = new Date(2026,10,20,12)
  inquire()
  act(vendor, 'channelQuote', quote(1500,'2026-11-19'))
  act(dealer, 'salesQuote', quote(1700))
  const quoteId = view(client).salesQuotes[0].id
  act(client,'confirmSales',{quoteId})
  assert.throws(() => act(dealer,'placeOrder',{quoteId},raw.revision,tomorrow), {code:'CONFLICT'})
  act(vendor,'channelQuote',quote(1501),raw.revision,tomorrow)
  assert.equal(view(client,tomorrow).salesQuotes[0].status,'superseded')
  assert.equal(view(client,tomorrow).stage,'processing')
  const revised = act(dealer,'salesQuote',quote(1750),raw.revision,tomorrow)
  const nextId = revised.salesQuotes.at(-1).id
  act(client,'confirmSales',{quoteId:nextId},raw.revision,tomorrow)
  assert.equal(act(dealer,'placeOrder',{quoteId:nextId},raw.revision,tomorrow).order.channelQuoteId,revised.channelQuotes.at(-1).id)
})

test('独立仓库实例共享演示变更，目录发布和重置不触及正式会话', async () => {
  const values = new Map([['auth-token','unchanged']])
  const storage = {getItem:key => values.get(key) ?? null,setItem:(key,value) => values.set(key,value)}
  const events = new EventTarget()
  const repo = role => createDemoRepository(role, {storage,events,now:() => start})
  const customerRepo = repo('customer'), dealerRepo = repo('reseller'), vendorRepo = repo('manufacturer')
  let notifications = 0
  const unsubscribe = dealerRepo.subscribe(() => notifications++)
  const count = (await dealerRepo.list()).length
  const entry = await customerRepo.create({ title:'线上新项目',requirement:'需要 15 个 ERP 席位' })
  assert.equal((await dealerRepo.get(entry.id)).isTodo,true)
  assert.equal(notifications,1)
  unsubscribe()
  const draft = await vendorRepo.saveProduct({name:'测试软件',modules:'库存',deployment:'云',license:'按席',published:false})
  assert.ok(!(await dealerRepo.listProducts()).some(p=>p.id === draft.id))
  await assert.rejects(dealerRepo.saveProduct(draft),{code:'FORBIDDEN'})
  await vendorRepo.saveProduct({...draft,published:true})
  assert.ok((await dealerRepo.listProducts()).some(p=>p.id === draft.id))
  await dealerRepo.reset()
  assert.equal((await dealerRepo.list()).length,count)
  assert.equal(values.get('auth-token'),'unchanged')
  assert.equal(notifications,1)
  values.set(demoStorageKey,'corrupt')
  await assert.rejects(dealerRepo.list(),/重置演示/)
  await vendorRepo.reset()
  assert.equal((await dealerRepo.list()).length,count)
})
