export function createHttpVisitRepository(request) {
  const base = '/api/channel/v1/visits'
  async function write(path, method, body, requestId) {
    try {
      return await request(path, { method, body: { ...body, requestId } })
    } catch (error) {
      if (error instanceof Error) error.requestId = requestId
      throw error
    }
  }
  return {
    async list() { return (await request(base)).items },
    get: id => request(base + '/' + encodeURIComponent(id)),
    save(id, payload, revision, requestId = crypto.randomUUID()) {
      return write(id ? base + '/' + encodeURIComponent(id) : base, id ? 'PUT' : 'POST', { revision, payload }, requestId)
    },
    confirm(id, revision, createOpportunity, requestId = crypto.randomUUID()) {
      return write(base + '/' + encodeURIComponent(id) + '/confirm', 'POST', { revision, confirmed: true, createOpportunity }, requestId)
    },
    remove(id, revision, requestId = crypto.randomUUID()) {
      return write(base + '/' + encodeURIComponent(id), 'DELETE', { revision }, requestId)
    },
  }
}
