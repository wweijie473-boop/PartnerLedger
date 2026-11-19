import assert from 'node:assert/strict'
import test from 'node:test'
import { clipboardTSV, parseAddress, parseClipboard, validateCell } from '../src/tables/cells.js'
import { displayCell } from '../src/tables/formulas.js'
import { createSaveQueue } from '../src/tables/saveQueue.js'

test('workbook coordinates, Excel multi-line clipboard, safe plain export', () => {
  assert.deepEqual(parseAddress('AA100000'), { row: 99999, col: 26 })
  assert.deepEqual(parseClipboard('型号\t数量\r\n"含\n服务"\t20\r\n'), [['型号', '数量'], ['含\n服务', '20']])
  assert.throws(() => parseClipboard('X\t'.repeat(250)), /250/)
  assert.equal(clipboardTSV([['=WEBSERVICE("url")', '-2']]), '"\'=WEBSERVICE(""url"")"\t-2')
  assert.equal(validateCell('date', '2026-02-29'), '请输入有效日期（YYYY-MM-DD）。')
})

test('bounded formulas evaluate ranges, detect cycles and do not evaluate unlisted functions', () => {
  const cells = [['20', '1200', '=A1*B1'], ['5', '4', '=SUM(A1:B2)'], ['=B3', '=A3', '=WEBSERVICE(foo)']]
  const cols = [0, 1, 2].map(() => ({ kind: 'auto' }))
  const display = (r, c) => displayCell(r, c, (row, col) => cells[row][col], cells.length, cols)
  assert.equal(display(0, 2), '24000')
  assert.equal(display(1, 2), '1229')
  assert.equal(display(2, 0), '#CYCLE!')
  assert.equal(display(2, 2), '#FORMULA!')
})

test('autosave uses the same key after a lost response and protects a newer version', async () => {
  const calls = []
  const queue = createSaveQueue({
    revision: 7, onChange() {}, onSaved() {},
    async save(batch) {
      calls.push(batch)
      if (calls.length === 1) throw Object.assign(new Error('lost response'), { status: 0 })
      return { revision: 8 }
    },
  })
  try {
    queue.add([{ row: 1, col: 1, input: 'value' }])
    assert.equal(await queue.flush(), false)
    assert.equal(await queue.retry(), true)
    assert.deepEqual(calls[0], calls[1])
    queue.add([{ row: 1, col: 1, input: 'updated' }])
    queue.observe(9)
    assert.equal(queue.state().error?.status, 409)
    assert.equal(queue.state().overlay.get('1:1'), 'updated')
    assert.equal(await queue.flush(), false)
    assert.equal(calls.length, 2)
  } finally { queue.stop() }
})
