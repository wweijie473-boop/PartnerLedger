import test from 'node:test'
import assert from 'node:assert/strict'
import { createHttpProjectRepository } from '../src/channel/httpProjects.js'

const base = '/api/channel/v1/projects'

test('collects cursor pages and returns the organization-scoped detail unchanged', async () => {
  const seen = []
  const detail = { id: 'P/1', salesQuotes: [] }
  const repo = createHttpProjectRepository(async path => {
    seen.push(path)
    if (path.endsWith('/P%2F1')) return detail
    const after = new URL(path, 'https://example.invalid').searchParams.get('after')
    return after === null
      ? { items: [detail], next_cursor: 'page/+2' }
      : { items: [detail, { id: 'P2' }], next_cursor: null }
  })
  assert.deepEqual(await repo.list(), [detail, { id: 'P2' }])
  assert.equal(await repo.get('P/1'), detail)
  assert.deepEqual(seen, [base + '?limit=50', base + '?limit=50&after=page%2F%2B2', base + '/P%2F1'])
})

test('serializes quote prices without rewriting input or silently rounding extra precision', async () => {
  const sent = []
  const repo = createHttpProjectRepository(async (path, options) => { sent.push([path, options]); return { id: 'P' } })
  const lines = [{ quantity: 2, unitPrice: 8.3 }, { quantity: 1, unitPrice: '0.05' }]
  const quote = { lines, validUntil: '2027-01-01' }
  await repo.act('P', 'channelQuote', quote, 8)
  const [path, { method, body }] = sent[0]
  assert.equal(path, base + '/P/actions/channelQuote')
  assert.equal(method, 'POST')
  assert.match(body.requestId, /^[a-f\d-]{36}$/i)
  assert.equal(body.revision, 8)
  assert.deepEqual(body.payload.lines.map(line => line.unitPrice), ['8.30', '0.05'])
  assert.equal(lines[0].unitPrice, 8.3)
  assert.throws(() => repo.act('P', 'salesQuote', { lines: [{ unitPrice: '8.333' }] }, 9), /单价/)
  assert.equal(sent.length, 1)
})

test('reuses client request IDs on a retry, preserves conflicts and does not fall back to local records', async () => {
  const sent = []
  const failure = Object.assign(new Error('revision conflict'), { status: 409 })
  const repo = createHttpProjectRepository(async (path, options) => {
    sent.push({ path, ...options })
    if (sent.length === 1 || path.endsWith('/actions/placeOrder')) throw failure
    return { id: 'P', revision: 1 }
  })
  await assert.rejects(repo.create({ title: 'ERP' }), error => error === failure && Boolean(error.requestId))
  const key = failure.requestId
  assert.deepEqual(await repo.create({ title: 'ERP' }, key), { id: 'P', revision: 1 })
  assert.deepEqual(sent[0], sent[1])
  assert.deepEqual(sent[0].body, { title: 'ERP', requestId: key })
  await assert.rejects(repo.act('P', 'placeOrder', { quoteId: 'Q1' }, 4, 'same-request'), { status: 409, requestId: 'same-request' })
  assert.deepEqual(sent[2].body, { revision: 4, payload: { quoteId: 'Q1' }, requestId: 'same-request' })
})

test('rejects a repeated cursor rather than looping indefinitely', async () => {
  let calls = 0
  const repo = createHttpProjectRepository(async () => { calls++; return { items: [], next_cursor: 'loop' } })
  await assert.rejects(repo.list(), /游标无效/)
  assert.equal(calls, 2)
})
