import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { aiDemoProject, matchDemoProducts, parseDemoRequirement, routeDemoWorkspace } from '../channel/aiInquiryDemo.js'
import { createWriteJournal } from './writeJournal.js'

const inquiryAction = 'submitInquiry'
export const workspaceSkills = Object.freeze({
  match_products: '产品匹配',
  prepare_inquiry: '准备询价草稿',
  submit_inquiry: '发送厂商询价',
  project_status: '项目进展',
})
const draftPath = id => '/api/channel/v1/projects/' + encodeURIComponent(id) + '/inquiry-draft'
const canInquire = project => project?.actions?.includes(inquiryAction)

function editableRequirement(requirement) {
  return {
    enterprise_type: requirement.enterprise_type ?? '',
    employee_count: requirement.employee_count ?? '',
    required_modules: (requirement.required_modules ?? []).join('、'),
    deployment_mode: requirement.deployment_mode ?? '',
    license_count: requirement.license_count ?? '',
    version: requirement.version ?? '',
  }
}

function normalizeRequirement(fields) {
  return {
    enterprise_type: fields.enterprise_type.trim(),
    employee_count: fields.employee_count || null,
    required_modules: [...new Set(fields.required_modules.split(/[、，,；;\n]/).map(item => item.trim()).filter(Boolean))],
    deployment_mode: fields.deployment_mode.trim(),
    license_count: fields.license_count || null,
    version: fields.version.trim(),
  }
}

export function useAiConversation(props, log) {
  const demo = Boolean(props.demoRepository)
  const repo = demo ? props.demoRepository : createHttpProjectRepository(props.request)
  const controller = new AbortController()
  const options = { signal: controller.signal }
  const post = (path, body, timeout) => props.request(path, { ...options, method: 'POST', body, timeout })
  const journal = demo ? null : createWriteJournal(
    'partnerledger-ai-write:' + props.organizationId + ':' + props.userId,
    ({ kind, input, requestId }) => kind === 'draft'
      ? props.request(draftPath(input.id), { method: 'POST', body: { ...input.body, requestId } })
      : repo.act(input.id, inquiryAction, input.body, input.revision, requestId),
  )
  const input = ref('')
  const turns = ref([])
  const projects = ref([])
  const selectedId = ref(props.initialProject)
  const selectedProject = computed(() => projects.value.find(item => item.id === selectedId.value))
  const inquiryProjects = computed(() => projects.value.filter(canInquire))
  const loadingProjects = ref(false)
  const projectError = ref('')
  const working = ref('')
  const approval = ref(null)
  const approvalOpen = ref(false)
  const approvalError = ref('')
  const choices = ref(null)
  const pendingWrite = computed(() => journal?.state.job)
  const recoveryError = computed(() => journal?.state.error)
  let turnId = 0
  let disposed = false

  onUnmounted(() => {
    disposed = true
    controller.abort()
  })

  function say(text, extra = {}) {
    if (disposed) return
    turns.value.push({ id: ++turnId, role: 'assistant', text, ...extra })
    nextTick(() => {
      if (log.value) log.value.scrollTop = log.value.scrollHeight
    })
  }

  async function loadProjects() {
    loadingProjects.value = true
    projectError.value = ''
    try {
      projects.value = await repo.list(options)
      if (selectedId.value && !selectedProject.value) {
        projectError.value = '当前项目暂不可访问，可以选择其他项目。'
        selectedId.value = ''
      }
    } catch (cause) {
      if (cause.name !== 'AbortError') projectError.value = '暂时无法读取项目：' + cause.message
    } finally {
      loadingProjects.value = false
    }
  }

  async function readDraft(id) {
    try {
      return demo ? repo.getInquiryDraft(id) : await props.request(draftPath(id), options)
    } catch (cause) {
      if (cause.status === 404) return null
      throw cause
    }
  }

  function getProduct(id) {
    return demo
      ? repo.listProducts().then(items => items.find(item => item.id === id))
      : props.request('/api/products/' + encodeURIComponent(id), options)
  }

  async function start() {
    await loadProjects()
    if (disposed || !selectedProject.value || !canInquire(selectedProject.value) || pendingWrite.value) return
    const id = selectedId.value
    try {
      const draft = await readDraft(id)
      if (!draft || draft.projectRevision !== selectedProject.value?.revision || selectedId.value !== id || turns.value.length) return
      const product = await getProduct(draft.productId)
      if (selectedId.value === id && !turns.value.length) {
        say('这个项目已有询价草稿。你可以查看内容，也可以让我准备发送审批。', { draft, product })
      }
    } catch (cause) {
      if (cause.name !== 'AbortError' && !turns.value.length) say('暂时无法读取该项目的已保存草稿。你可以继续对话或稍后重试。')
    }
  }

  onMounted(start)

  function projectFromText(text) {
    const matching = projects.value.filter(project => text.includes(project.title) ||
      (demo ? text.includes(project.id) : new RegExp('(?:^|[^0-9])(?:#|项目\\s*)' + project.id + '(?![0-9])').test(text)))
    if (matching.length > 1) throw new Error('这条消息提到了多个项目，请在上方选择项目，再单独告诉我处理哪个。')
    const project = matching[0]
    if (project) selectedId.value = project.id
    if (!project && /(?:项目\s*#?\s*\d+|#\d+|PRJ-\d+)/i.test(text)) {
      throw new Error('找不到这条消息所指的项目。请从上方选择有权访问的项目。')
    }
    return project ?? selectedProject.value
  }

  function requestApproval(value, text) {
    approval.value = value
    approvalError.value = ''
    approvalOpen.value = true
    say(text, { approval: value })
  }

  function closeApproval() {
    approvalOpen.value = false
  }

  function reopenApproval() {
    if (approval.value) approvalOpen.value = true
  }

  function declineApproval() {
    approvalOpen.value = false
    approval.value = null
    say('已拒绝本次审批，相关操作未执行。')
  }

  async function proposeSend(project, draft, product) {
    if (!canInquire(project) || draft.projectRevision !== project.revision) {
      say('项目版本或状态已变化。请打开项目核对后重新发起。', { project })
      return
    }
    product ??= await getProduct(draft.productId)
    if (!product?.published) {
      say('草稿里的产品已不可用，请重新准备询价。', { project })
      return
    }
    requestApproval({
      kind: 'send',
      title: '审批发送厂商询价',
      project,
      product,
      summary: draft.summary,
    }, '我已准备好发给厂商的内容。请在弹窗核对产品和摘要，批准后我会提交询价。')
  }

  async function proposeRequirement(intent, source, project) {
    working.value = '正在提取需求字段'
    const data = demo ? parseDemoRequirement(source) : await post('/api/ai/parse-requirement', { text: source }, 32000)
    if (disposed) return
    requestApproval({
      kind: 'requirement',
      title: '审批需求字段',
      intent,
      source,
      projectId: canInquire(project) ? project.id : '',
      fields: editableRequirement(data),
      questions: data.follow_up_questions ?? [],
    }, '我识别到「' + workspaceSkills[intent] + '」。请在弹窗核对字段，未知项可以留空。')
  }

  async function dispatch(value = input.value) {
    const text = value.trim()
    if (!text || working.value || loadingProjects.value || pendingWrite.value) return
    if (approval.value) { reopenApproval(); return }
    if ([...text].length > 2000) { say('单条消息限 2000 字，请缩短后发送。'); return }
    input.value = ''
    choices.value = null
    say(text, { role: 'user' })
    working.value = '正在识别所需技能'
    try {
      const route = demo ? routeDemoWorkspace(text) : await post('/api/ai/workspace/route', { text }, 32000)
      if (disposed) return
      if (!Object.hasOwn(workspaceSkills, route.intent)) {
        say('我可以匹配软件产品、准备或发送厂商询价，也可以查看项目进展。告诉我你想办理哪一项。')
        return
      }
      working.value = '正在读取业务上下文'
      const context = projectFromText(text)
      const project = context ? await repo.get(context.id, options) : null
      if (project) projects.value = projects.value.map(item => item.id === project.id ? project : item)
      if (route.intent === 'project_status') {
        if (!project) say('请先选择一个项目，或在消息里写出项目名称。')
        else say('这是「' + project.title + '」的当前进展。', { project })
        return
      }
      if (route.intent === 'submit_inquiry') {
        if (!project || !canInquire(project)) {
          say('请先选择一个仍待询价的项目。', { project })
          return
        }
        const draft = await readDraft(project.id)
        if (draft) {
          await proposeSend(project, draft)
          return
        }
      }
      if (project && route.intent !== 'match_products' && !canInquire(project)) {
        say('当前项目已经进入后续阶段。请选择另一个待询价项目。', { project })
        return
      }
      const source = route.use_project_requirement ? project?.requirement || (demo ? aiDemoProject.requirement : '') : text
      if (!source) {
        say('请先选择含采购需求的项目，或直接告诉我所需模块、部署方式等条件。')
        return
      }
      await proposeRequirement(route.intent, source, project)
    } catch (cause) {
      if (cause.name !== 'AbortError') {
        const modelUnavailable = working.value !== '正在读取业务上下文' && [502, 503, 504].includes(cause.status)
        say(cause.message || '技能暂时不可用，请稍后重试。', modelUnavailable ? { manualSource: text } : {})
      }
    } finally {
      working.value = ''
    }
  }

  function fillManually(source) {
    if (working.value || approval.value || pendingWrite.value) return
    choices.value = null
    requestApproval({
      kind: 'requirement',
      title: '审批需求字段',
      intent: 'prepare_inquiry',
      source: selectedProject.value?.requirement || source,
      projectId: canInquire(selectedProject.value) ? selectedId.value : '',
      fields: editableRequirement({}),
      questions: [],
    }, '请填写已知需求。批准后，我会继续自动匹配和准备询价草稿。')
  }

  async function saveApprovedDraft(task, product) {
    const body = { revision: task.project?.revision, confirmed: true, productId: product.id, requirement: task.requirement }
    working.value = '正在保存询价草稿'
    const draft = demo
      ? await repo.saveInquiryDraft(task.project?.id, body, { ...aiDemoProject, requirement: task.source })
      : await journal.run('draft', { id: task.project.id, body })
    const project = task.project ?? await repo.get(draft.projectId, options)
    if (!projects.value.some(item => item.id === project.id)) projects.value = [project, ...projects.value]
    selectedId.value = project.id
    say('已自动保存询价草稿，项目仍待询价。', { draft, product })
    if (task.intent === 'submit_inquiry') await proposeSend(project, draft, product)
  }

  async function executeRequirements(item, requirement, projectId) {
    working.value = '审批已通过，正在匹配产品'
    const wantsDraft = item.intent !== 'match_products'
    const project = wantsDraft && projectId ? await repo.get(projectId, options) : null
    if (wantsDraft && (project ? !canInquire(project) || project.revision !== item.revision : !demo)) {
      say('项目状态或版本已变化，请重新发起审批。', { project })
      return
    }
    const matches = demo
      ? matchDemoProducts(await repo.listProducts(), requirement)
      : (await post('/api/ai/match-products', requirement)).items
    if (disposed) return
    const eligible = matches.filter(item => item.status !== 'mismatch')
    const exact = eligible.filter(item => item.status === 'matched')
    const automatic = eligible.length === 1 ? eligible[0] : exact.length === 1 ? exact[0] : null
    if (!eligible.length) {
      say('没有符合已知条件的候选。可以补充或调整需求，再让我匹配一次。', { matches })
      return
    }
    if (!wantsDraft) {
      say('匹配完成，找到 ' + eligible.length + ' 款可进一步核对的产品。', { matches })
      return
    }
    const task = { project, source: item.source, requirement, intent: item.intent }
    if (automatic) {
      say('匹配完成，选用「' + automatic.product.name + '」准备询价。', { matches: [automatic] })
      await saveApprovedDraft(task, automatic.product)
    } else {
      choices.value = { task, eligible }
      say('有多款候选，尚不能确定询价对象。选定产品后，我会自动保存草稿。', { matches: eligible, choices: choices.value })
    }
  }

  async function approve() {
    const item = approval.value
    if (!item || working.value || pendingWrite.value) return
    approvalError.value = ''
    let requirement
    if (item.kind === 'requirement') {
      requirement = normalizeRequirement(item.fields)
      if (!Object.values(requirement).some(value => Array.isArray(value) ? value.length : value)) {
        approvalError.value = '请至少填写一个已知需求字段。'
        return
      }
      if (requirement.required_modules.length > 30 || requirement.required_modules.some(module => [...module].length > 70)) {
        approvalError.value = '模块最多 30 项，每项不超过 70 字。'
        return
      }
      if (!demo && item.intent !== 'match_products' && !item.projectId) {
        approvalError.value = '请先关联待询价项目。'
        return
      }
      if (item.projectId) item.revision = inquiryProjects.value.find(project => project.id === item.projectId)?.revision
    }
    approval.value = null
    approvalOpen.value = false
    try {
      if (item.kind === 'requirement') {
        await executeRequirements(item, requirement, item.projectId)
      } else {
        working.value = '审批已通过，正在发送询价'
        const body = { product: item.product.id, summary: item.summary }
        const { id, revision } = item.project
        const result = demo ? await repo.act(id, inquiryAction, body, revision)
          : await journal.run(inquiryAction, { id, revision, body })
        projects.value = projects.value.map(project => project.id === result.id ? result : project)
        say('询价已提交给厂商。', { project: result })
      }
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  function reportExecutionError(cause) {
    if (cause.name === 'AbortError') return
    if (pendingWrite.value) say('提交结果待确认。请用原请求确认结果，避免重复办理。')
    else if (cause.status === 409 || cause.code === 'CONFLICT') say('记录已更新，请查看项目现状并重新发起。')
    else say(cause.message || '这次执行未完成，请查看项目后再试。')
  }

  async function useCandidate(choice, product) {
    if (working.value || !choices.value || choices.value !== choice || approval.value || pendingWrite.value) return
    choices.value = null
    try {
      await saveApprovedDraft(choice.task, product)
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  async function reviewSavedDraft(turn) {
    if (working.value || approval.value || pendingWrite.value) return
    working.value = '正在核对项目和草稿'
    choices.value = null
    try {
      const project = await repo.get(turn.draft.projectId, options)
      const draft = await readDraft(project.id)
      if (!draft || draft.productId !== turn.draft.productId || draft.summary !== turn.draft.summary ||
          draft.projectRevision !== turn.draft.projectRevision) {
        say('这张卡片的草稿已更新或项目不再待询价，请重新查询项目。', { project })
        return
      }
      await proposeSend(project, draft)
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  async function recoverWrite() {
    if (!pendingWrite.value || working.value) return
    const job = pendingWrite.value
    working.value = '正在用原请求确认结果'
    try {
      const result = await journal.retry()
      if (job.kind === 'draft') {
        say('已确认草稿保存成功。', { draft: result })
      } else {
        say('已确认询价已提交。', { project: result })
      }
      selectedId.value = job.input.id
      await loadProjects()
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  return {
    demo, input, turns, projects, selectedId, inquiryProjects, loadingProjects, projectError, working,
    approval, approvalOpen, approvalError, choices, pendingWrite, recoveryError,
    loadProjects, dispatch, fillManually, closeApproval, reopenApproval, declineApproval, approve, useCandidate, reviewSavedDraft, recoverWrite,
  }
}
