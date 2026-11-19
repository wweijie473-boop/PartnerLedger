// Proposed contract. Enable in the authenticated dealer workspace only after server-side authorization is available.
export function createHttpOpportunityRepository(request) {
  const base = '/api/channel/v1/opportunities'
  async function write(path, body, requestId = crypto.randomUUID()) {
    try { return await request(path, { method:'POST', body:{ ...body, requestId } }) }
    catch (cause) { if (cause instanceof Error) cause.requestId = requestId; throw cause }
  }
  return {
    opportunityDesk: () => request(base + '/desk'),
    captureOpportunity: (projectId, payload, requestId) => write(base, { projectId, payload }, requestId),
    addPartner: (payload, requestId) => write(base + '/partners', { payload }, requestId),
    actOpportunity: (id, action, payload, revision, requestId) =>
      write(base + '/' + encodeURIComponent(id) + '/actions/' + encodeURIComponent(action), { payload, revision }, requestId),
  }
}
