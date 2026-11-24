const base = '/api/channel/v1/projects'

function yuan(value) {
  const parts = /^(\d{1,8})(?:\.(\d{1,2}))?$/.exec(String(value))
  if (!parts) throw new RangeError('无效的人民币单价。')
  return String(Number(parts[1])) + '.' + (parts[2] ?? '').padEnd(2, '0')
}

export function createHttpProjectRepository(request) {
  async function post(path, body, requestId) {
    try {
      return await request(path, { method: 'POST', body: { ...body, requestId } })
    } catch (error) {
      if (error instanceof Error) error.requestId = requestId
      throw error
    }
  }
  return {
    async workspace({ scope, stage, keyword, after, limit, signal }) {
      const params = new URLSearchParams({ scope, limit: String(limit) })
      if (stage) params.set('stage', stage)
      if (keyword) params.set('q', keyword.trim())
      if (after) params.set('after', after)
      const page = await request(base + '/workspace?' + params, { signal })
      if (!Array.isArray(page?.items) || !Array.isArray(page.stages) ||
          !page.items.every(item => typeof item?.isTodo === 'boolean' &&
            ['id', 'title', 'stage', 'stageLabel', 'nextStep'].every(key => typeof item[key] === 'string')) ||
          !page.stages.every(option => typeof option?.id === 'string' && typeof option.label === 'string') ||
          !['all', 'mine', 'partners'].every(key => Number.isSafeInteger(page.counts?.[key]) && page.counts[key] >= 0) ||
          (page.next_cursor !== null && (typeof page.next_cursor !== 'string' || !page.next_cursor || page.next_cursor === after))) {
        throw new Error('项目工作台响应无效。')
      }
      return page
    },
    async list({ signal } = {}) {
      const projects = new Map()
      const cursors = new Set()
      const params = new URLSearchParams({ limit: '50' })
      while (true) {
        const page = await request(base + '?' + params, signal ? { signal } : undefined)
        if (!Array.isArray(page?.items) || !Object.hasOwn(page, 'next_cursor')) throw new Error('项目列表响应无效。')
        for (const project of page.items) projects.set(project.id, project)
        if (page.next_cursor === null) return Array.from(projects.values())
        const cursor = page.next_cursor
        if (typeof cursor !== 'string' || !cursor || cursors.has(cursor)) throw new Error('项目列表分页游标无效。')
        cursors.add(cursor)
        params.set('after', cursor)
      }
    },
    get: (id, { signal } = {}) => request(base + '/' + encodeURIComponent(id), signal ? { signal } : undefined),
    async opportunities(id, { before, signal } = {}) {
      const query = before ? '?before=' + encodeURIComponent(before) : ''
      const page = await request(base + '/' + encodeURIComponent(id) + '/opportunities' + query, { signal })
      const key = value => typeof value === 'string' && /^[1-9]\d*$/.test(value)
      if (!Array.isArray(page?.items) ||
          !page.items.every(item => item && key(item.id) && typeof item.note === 'string' && typeof item.stage === 'string') ||
          (page.next_cursor !== null && (!key(page.next_cursor) || page.next_cursor === before))) {
        throw new Error('项目附加商机响应无效。')
      }
      return page
    },
    create(payload, requestId = crypto.randomUUID()) {
      return post(base, payload, requestId)
    },
    act(id, action, payload, revision, requestId = crypto.randomUUID()) {
      const data = ['channelQuote', 'salesQuote'].includes(action)
        ? { ...payload, lines: payload.lines.map(line => ({ ...line, unitPrice: yuan(line.unitPrice) })) }
        : payload
      return post(base + '/' + encodeURIComponent(id) + '/actions/' + encodeURIComponent(action), { revision, payload: data }, requestId)
    },
  }
}
