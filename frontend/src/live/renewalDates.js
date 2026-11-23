const dateInChina = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' })

export function renewalDays(dueAt, now = new Date()) {
  const due = dateInChina.format(new Date(dueAt))
  const today = dateInChina.format(now)
  return (Date.parse(due) - Date.parse(today)) / 86400000
}
