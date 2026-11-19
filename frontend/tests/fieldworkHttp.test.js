import test from 'node:test'
import assert from 'node:assert/strict'
import { createHttpVisitRepository } from '../src/live/httpVisits.js'
import { createWriteJournal } from '../src/live/writeJournal.js'

function memoryStorage() {
  const entries = new Map()
  return { getItem:key => entries.get(key) || null, setItem:(key,value) => entries.set(key,value), removeItem:key => entries.delete(key) }
}

test('拜访创建、编辑、确认和删除使用真实接口的版本与显式确认字段', async () => {
  const calls = []
  const repo = createHttpVisitRepository(async (path,options) => {
    calls.push({path,...options})
    return path.endsWith('/visits') && !options ? {items:[{id:'41'}]} : {id:'41',revision:2}
  })
  assert.deepEqual(await repo.list(),[{id:'41'}])
  await repo.save('',{text:'ERP已定，另需对接'},0,'create-key')
  await repo.save('41',{text:'已核对',reviewed:true},2,'save-key')
  await repo.confirm('41',3,true,'confirm-key')
  await repo.remove('41',4,'remove-key')
  assert.equal(calls[1].method,'POST')
  assert.equal(calls[1].body.revision,0)
  assert.equal(calls[2].method,'PUT')
  assert.equal(calls[2].path,'/api/channel/v1/visits/41')
  assert.deepEqual(calls[3].body,{revision:3,confirmed:true,createOpportunity:true,requestId:'confirm-key'})
  assert.equal(calls[4].method,'DELETE')
  assert.deepEqual(calls[4].body,{revision:4,requestId:'remove-key'})
})

test('响应丢失后刷新页面，保留原请求内容及编号，其他账号不可继承', async () => {
  const storage = memoryStorage()
  const sent = []
  const first = createWriteJournal('org:member', async job => {
    sent.push(JSON.parse(JSON.stringify(job)))
    throw Object.assign(new Error('响应丢失'),{status:0})
  },storage)
  const payload = {id:'41',revision:3,createOpportunity:true}
  await assert.rejects(first.run('confirm',payload))
  payload.revision = 900
  await assert.rejects(first.run('confirm',payload),/上一笔/)
  assert.equal(createWriteJournal('org:other',async () => {},storage).state.job,null)
  const restored = createWriteJournal('org:member',async job => { sent.push(job); return {opportunityId:'17'} },storage)
  assert.deepEqual(await restored.retry(),{opportunityId:'17'})
  assert.deepEqual(sent[0],sent[1])
  assert.equal(sent[1].input.revision,3)
  assert.equal(restored.state.job,null)
  assert.equal(storage.getItem('org:member'),null)
})

test('版本冲突结束原提交，允许用户核对后用新编号重新办理', async () => {
  const storage = memoryStorage()
  const ids = []
  const journal = createWriteJournal('member',async job => {
    ids.push(job.requestId)
    if (ids.length === 1) throw Object.assign(new Error('版本已更新'),{status:409,code:'revision_conflict'})
    return {revision:6}
  },storage)
  await assert.rejects(journal.run('save',{revision:3}))
  assert.equal(journal.state.job,null)
  assert.match(journal.state.error,/版本已更新/)
  await journal.run('save',{revision:5})
  assert.notEqual(ids[0],ids[1])
})
