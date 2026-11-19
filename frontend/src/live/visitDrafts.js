import { suggestOpportunity } from '../channel/opportunities.js'

export function splitVisit(text) {
  const parts = text.split(/[，,；;。\n]/).map(value => value.trim()).filter(Boolean)
  const extra = parts.filter(value => /另外|还需|还要|额外|另需|还想|也需要/.test(value))
  return {
    main:parts.filter(value => !extra.includes(value)).join('；'),
    additional:extra.join('；'),
    ...suggestOpportunity(extra.join('；')),
  }
}

export function createVisitDraftStore(scope, storage = window.localStorage) {
  const key = 'partnerledger-visit-drafts:' + scope
  function list() {
    const data = storage.getItem(key)
    if (!data) return []
    const rows = JSON.parse(data)
    if (!Array.isArray(rows)) throw new Error('拜访草稿无法读取，请保留本机数据后检查。')
    return rows
  }
  return {
    list,
    clear() { storage.removeItem(key) },
    save(input, expectedVersion) {
      const rows = list()
      const previous = rows.find(item => item.id === input.id)
      if ((previous?.revision || 0) !== expectedVersion) throw new Error('草稿已在其他页面更新，请重新打开后再保存。')
      const record = { ...input, id:input.id || crypto.randomUUID(), revision:expectedVersion + 1, updatedAt:new Date().toISOString() }
      storage.setItem(key,JSON.stringify([record,...rows.filter(item => item.id !== record.id)]))
      return record
    },
    remove(id) {
      storage.setItem(key,JSON.stringify(list().filter(item => item.id !== id)))
    },
  }
}
