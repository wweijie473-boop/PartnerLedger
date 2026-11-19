export const MAX_CHANGES = 250
export const MAX_ROWS = 100_000
export const MAX_COLS = 128
export const ROW_BLOCK = 40
export const COL_BLOCK = 12
export const keyFor = (row, col) => row + ':' + col

const numeric = /^-?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)$/
export const isDecimal = value => numeric.test(value)
const encoder = new TextEncoder()

export function columnName(index) {
  let result = ''
  for (let n = index + 1; n > 0; n = Math.floor((n - 1) / 26)) {
    result = String.fromCharCode(65 + (n - 1) % 26) + result
  }
  return result
}

export function validateCell(kind, value) {
  if (encoder.encode(value).length > 1000) return '单元格内容超过 1000 字节。'
  if (value === '' || kind === 'auto' || kind === 'text') return ''
  if (kind === 'number' && (numeric.test(value) || value.startsWith('='))) return ''
  if (kind === 'checkbox' && /^(true|false)$/.test(value)) return ''
  if (kind === 'date' && /^(?!0000)\d{4}-\d{2}-\d{2}$/.test(value)) {
    const date = new Date(value + 'T00:00:00Z')
    if (Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value) return ''
  }
  return { number: '请输入十进制数字或公式。', date: '请输入有效日期（YYYY-MM-DD）。', checkbox: '请输入 true 或 false。' }[kind]
}

export function parseClipboard(text) {
  if (text.length > MAX_CHANGES * 1000 + 1000) throw new Error('粘贴内容过大。')
  const rows = [[]]
  let value = '', quoted = false, closed = false, count = 0
  const finish = () => {
    if (++count > MAX_CHANGES) throw new Error('单次最多粘贴 250 个单元格。')
    rows.at(-1).push(value)
    value = ''
    closed = false
  }
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quoted) {
      if (ch === '"' && text[i + 1] === '"') { value += '"'; i++ }
      else if (ch === '"') { quoted = false; closed = true }
      else value += ch
    } else if (ch === '\t' || ch === '\r' || ch === '\n') {
      finish()
      if (ch === '\t') continue
      if (ch === '\r' && text[i + 1] === '\n') i++
      if (i < text.length - 1) rows.push([])
      else return rows
    } else if (ch === '"' && !value && !closed) quoted = true
    else if (closed) throw new Error('粘贴内容的引号格式无效。')
    else value += ch
  }
  if (quoted) throw new Error('粘贴内容的引号格式无效。')
  finish()
  return rows
}

export function clipboardTSV(matrix) {
  return matrix.map(row => row.map(value => {
    if (/^[\s\uFEFF]*[=+@-]/u.test(value) && !isDecimal(value.trim()) || /^[\t\r\n]/.test(value)) value = "'" + value
    return /[\t\n\r"]/.test(value) ? '"' + value.replaceAll('"', '""') + '"' : value
  }).join('\t')).join('\r\n')
}

export const internalClipboard = 'application/x-partnerledger-table+json'

export function readClipboard(data) {
  const internal = data.getData(internalClipboard)
  if (!internal || internal.length > 1_600_000) return parseClipboard(data.getData('text/plain'))
  const rows = JSON.parse(internal)
  if (!Array.isArray(rows) || rows.length > MAX_CHANGES ||
    rows.some(row => !Array.isArray(row) || !row.length || row.some(value => typeof value !== 'string')) ||
    rows.reduce((count, row) => count + row.length, 0) > MAX_CHANGES) throw new Error('复制区域无效。')
  return rows
}

export function parseAddress(value) {
  const match = /^([A-Z]{1,3})([1-9][0-9]{0,5})$/i.exec(value.trim())
  if (!match) return null
  let col = 0
  for (const letter of match[1].toUpperCase()) col = col * 26 + letter.charCodeAt(0) - 64
  return { row: Number(match[2]) - 1, col: col - 1 }
}
