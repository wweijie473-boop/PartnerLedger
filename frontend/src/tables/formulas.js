import { isDecimal, keyFor } from './cells.js'

const token = /\s*([A-Z]{1,3}[1-9]\d{0,5}|[A-Z]+|\d+(?:\.\d+)?|\.\d+|[()+*/,:-])/iy
const reference = /^([A-Z]+)([0-9]+)$/
const functions = {
  SUM: values => values.reduce((sum, value) => sum + value, 0),
  AVERAGE: values => values.length ? functions.SUM(values) / values.length : NaN,
  COUNT: values => values.length,
  MIN: values => values.length ? Math.min(...values) : 0,
  MAX: values => values.length ? Math.max(...values) : 0,
}

function point(ref) {
  const match = reference.exec(ref)
  if (!match) throw '#REF!'
  let col = 0
  for (const letter of match[1]) col = col * 26 + letter.charCodeAt(0) - 64
  return [Number(match[2]) - 1, col - 1]
}

function tokenize(formula) {
  const input = formula.slice(1).trim()
  const tokens = []
  while (tokens.length < 256 && (tokens.length === 0 || token.lastIndex < input.length)) {
    if (tokens.length === 0) token.lastIndex = 0
    const match = token.exec(input)
    if (!match) throw '#FORMULA!'
    tokens.push(match[1].toUpperCase())
  }
  if (token.lastIndex !== input.length) throw '#LIMIT!'
  return tokens
}

function calculate(formula, read, spend) {
  const tokens = tokenize(formula)
  let cursor = 0, depth = 0
  const peek = () => tokens[cursor]
  const take = expected => {
    if (tokens[cursor] !== expected) throw '#FORMULA!'
    cursor++
  }
  function expression() {
    let value = term()
    while (peek() === '+' || peek() === '-') {
      const op = tokens[cursor++]
      const next = term()
      value = op === '+' ? value + next : value - next
    }
    return value
  }
  function term() {
    let value = scalar()
    while (peek() === '*' || peek() === '/') {
      const op = tokens[cursor++]
      const next = scalar()
      if (op === '/' && next === 0) throw '#DIV/0!'
      value = op === '*' ? value * next : value / next
    }
    return value
  }
  function scalar() {
    if (++depth > 32) throw '#LIMIT!'
    try {
      const item = tokens[cursor++]
      if (item === '+' || item === '-') return (item === '-' ? -1 : 1) * scalar()
      if (item === '(') {
        const value = expression()
        take(')')
        return value
      }
      if (reference.test(item ?? '')) {
        const value = read(...point(item))
        if (value === null) return 0
        if (typeof value !== 'number') throw '#VALUE!'
        return value
      }
      if (item && Object.hasOwn(functions, item)) return aggregate(item)
      if (item && /^(?:\d|\.)/.test(item)) return Number(item)
      throw '#FORMULA!'
    } finally { depth-- }
  }
  function aggregate(name) {
    const values = []
    take('(')
    while (true) {
      if (reference.test(peek() ?? '') && [':', ',', ')'].includes(tokens[cursor + 1])) {
        const [row, col] = point(tokens[cursor++])
        let endRow = row, endCol = col
        if (peek() === ':') {
          cursor++
          ;[endRow, endCol] = point(tokens[cursor++])
        }
        const height = Math.abs(endRow - row) + 1, width = Math.abs(endCol - col) + 1
        if (height * width > 256) throw '#LIMIT!'
        for (let r = Math.min(row, endRow); r <= Math.max(row, endRow); r++) {
          for (let c = Math.min(col, endCol); c <= Math.max(col, endCol); c++) {
            spend()
            const value = read(r, c)
            if (typeof value === 'number') values.push(value)
          }
        }
      } else {
        values.push(expression())
      }
      if (peek() !== ',') break
      cursor++
    }
    take(')')
    return functions[name](values)
  }
  const result = expression()
  if (cursor !== tokens.length || !Number.isFinite(result)) throw '#FORMULA!'
  return result
}

export function displayCell(row, col, read, rowCount, columns) {
  const cycle = new Set()
  const memo = new Map()
  let budget = 750
  function spend() { if (--budget < 0) throw '#LIMIT!' }
  function valueAt(r, c) {
    spend()
    if (r < 0 || c < 0 || r >= rowCount || c >= columns.length) throw '#REF!'
    const key = keyFor(r, c)
    if (cycle.has(key)) throw '#CYCLE!'
    if (memo.has(key)) return memo.get(key)
    const raw = read(r, c)
    if (raw === undefined) throw '#LOADING'
    const kind = columns[c].kind
    let value = raw === '' ? null : raw
    if ((kind === 'auto' || kind === 'number') && raw.startsWith('=')) {
      if (cycle.size >= 24) throw '#LIMIT!'
      cycle.add(key)
      try { value = calculate(raw, valueAt, spend) } finally { cycle.delete(key) }
    } else if (kind !== 'text' && kind !== 'date' && isDecimal(raw) && Number.isFinite(Number(raw))) {
      value = Number(raw)
    }
    memo.set(key, value)
    return value
  }
  const input = read(row, col)
  if (input === undefined || !input.startsWith('=') || !['auto', 'number'].includes(columns[col].kind)) return input ?? ''
  try {
    const value = valueAt(row, col)
    return typeof value === 'number' ? String(Number(value.toPrecision(12))) : String(value ?? '')
  } catch (error) {
    if (error === '#LOADING') return '…'
    return typeof error === 'string' ? error : '#FORMULA!'
  }
}
