import { createWriteJournal } from './writeJournal.js'

export function createProjectWriteJournal(repo, organizationId, userId, storage) {
  const key = `partnerledger-project-write:${organizationId}:${userId}`
  return createWriteJournal(key, ({ kind, input, requestId }) => kind === 'create'
    ? repo.create(input.payload, requestId)
    : repo.act(input.id, kind, input.payload, input.revision, requestId), storage)
}
