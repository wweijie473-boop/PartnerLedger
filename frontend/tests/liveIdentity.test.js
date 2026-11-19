import test from 'node:test'
import assert from 'node:assert/strict'
import { organizationRole } from '../src/live/entrances.js'
import { ApiError } from '../src/api.js'

test('正式身份支持三种组织，同时拒绝会话资料与组织响应不一致', () => {
  for (const [kind, role] of [['CUSTOMER','customer'],['RESELLER','dealer'],['MANUFACTURER','vendor']]) {
    const organization = { id:'12', kind, name:'示例组织' }
    const profile = { username:'member', organization:{...organization} }
    assert.equal(organizationRole(profile,organization),role)
    assert.equal(organizationRole(profile,{...organization,id:'13'}),null)
    assert.equal(organizationRole(profile,{...organization,kind:'UNSUPPORTED'}),null)
  }
})

test('保留版本和幂等冲突的错误码，办理失败时可以正确提示核对', () => {
  const revision = new ApiError(409,'revision_conflict')
  assert.equal(revision.status,409)
  assert.equal(revision.code,'revision_conflict')
  assert.match(revision.message,/协作方更新/)
  const retry = new ApiError(409,'idempotency_conflict')
  assert.match(retry.message,/内容已变更/)
})
