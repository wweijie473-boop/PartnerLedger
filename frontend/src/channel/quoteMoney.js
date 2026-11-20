export function quoteCents(value) {
  const parts = /^(\d+)(?:\.(\d{1,2}))?$/.exec(String(value))
  if (!parts) return 0n
  return BigInt(parts[1]) * 100n + BigInt((parts[2] || '').padEnd(2, '0'))
}

export function lineCents(line) {
  const quantity = Number(line.quantity)
  return Number.isSafeInteger(quantity) && quantity > 0 ? BigInt(quantity) * quoteCents(line.unitPrice) : 0n
}

export function formatCents(cents) {
  const whole = (cents / 100n).toLocaleString('zh-CN')
  const fraction = (cents % 100n).toString().padStart(2, '0')
  return '¥' + whole + '.' + fraction
}
