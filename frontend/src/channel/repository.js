import { actOnProject, createProject, demoActor, listProducts, projectList, saveProduct, seedState, viewProject } from './domain.js'
import { actOpportunity, addPartner, captureOpportunity, ensureOpportunityMemory, opportunityDesk } from './opportunities.js'

export const demoStorageKey = 'partnerledger-channel-demo-v1'
const changeEvent = demoStorageKey + ':changed'

export function createDemoRepository(role, options = {}) {
  const actor = demoActor(role)
  const clock = options.now ?? (() => new Date())
  const storage = options.storage ?? window.localStorage
  const events = options.events ?? globalThis.window
  const locks = options.locks ?? globalThis.window?.navigator?.locks
  function save(state) {
    try { storage.setItem(demoStorageKey, JSON.stringify(state)) }
    catch { throw new Error('无法保存本地演示，请检查浏览器存储空间与权限。') }
  }
  function read() {
    const raw = storage.getItem(demoStorageKey)
    if (!raw) {
      const state = seedState(clock())
      ensureOpportunityMemory(state, clock())
      save(state)
      return state
    }
    try {
      const state = JSON.parse(raw)
      if (state.schema === 1 && Array.isArray(state.projects) && Array.isArray(state.products)) return state
    } catch {}
    throw new Error('本地演示数据无法读取，可通过重置演示恢复。')
  }
  async function mutate(operation, reset = false) {
    const transaction = () => {
      const now = clock(), state = reset ? seedState(now) : read()
      const result = operation(state, now)
      save(state)
      events?.dispatchEvent(new Event(changeEvent))
      return result
    }
    return locks?.request ? locks.request(demoStorageKey, transaction) : transaction()
  }
  return {
    async list() { return projectList(read(), actor, clock()) },
    async get(id) { return viewProject(read().projects.find(item => item.id === id), actor, clock()) },
    create: payload => mutate((state, now) => createProject(state, actor, payload, now)),
    act: (id, action, payload, revision) => mutate((state, now) => actOnProject(state, actor, id, action, payload, revision, now)),
    async listProducts() { return listProducts(read(), actor) },
    saveProduct: payload => mutate(state => saveProduct(state, actor, payload)),
    async opportunityDesk() { return opportunityDesk(read(), actor, clock()) },
    captureOpportunity: (id, payload) => mutate((state, now) => captureOpportunity(state, actor, id, payload, now)),
    addPartner: payload => mutate((state, now) => addPartner(state, actor, payload, now)),
    actOpportunity: (id, action, payload, revision) => mutate((state, now) => actOpportunity(state, actor, id, action, payload, revision, now)),
    reset: () => mutate((state, now) => { ensureOpportunityMemory(state, now) }, true),
    subscribe(callback) {
      const external = event => { if (event.key === demoStorageKey || event.key === null) callback() }
      events?.addEventListener('storage', external)
      events?.addEventListener(changeEvent, callback)
      return () => {
        events?.removeEventListener('storage', external)
        events?.removeEventListener(changeEvent, callback)
      }
    },
  }
}
