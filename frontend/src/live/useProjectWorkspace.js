import { computed, ref } from 'vue'

export function useProjectWorkspace(repo, { overview, scope, selected, saved }) {
  const page = ref(null)
  const rows = computed(() => page.value?.items || [])
  const counts = computed(() => page.value?.counts || { all: 0, mine: 0, partners: 0 })
  const loading = ref(!selected.value)
  const loadError = ref('')
  const keyword = ref(saved?.keyword || '')
  const stage = ref(saved?.stage || '')
  const stageLabel = ref(saved?.stageLabel || '')
  const cursors = ref(saved?.cursors || [''])
  const stages = computed(() => {
    const options = page.value?.stages || []
    return stage.value && stageLabel.value && !options.some(option => option.id === stage.value)
      ? [...options, { id: stage.value, label: stageLabel.value }]
      : options
  })
  let request
  let searchTimer

  function cancelWorkspaceLoad() {
    clearTimeout(searchTimer)
    request?.abort()
  }

  async function loadWorkspace({ foreground = false } = {}) {
    if (selected.value || (request && !foreground)) return
    cancelWorkspaceLoad()
    const controller = new AbortController()
    request = controller
    if (foreground) loading.value = true
    try {
      const result = await repo.workspace({
        scope: overview ? 'mine' : scope.value, limit: overview ? 5 : 20,
        stage: stage.value, keyword: keyword.value, after: cursors.value.at(-1), signal: controller.signal,
      })
      if (!controller.signal.aborted) {
        page.value = result
        loadError.value = ''
        return true
      }
    } catch (error) {
      if (!controller.signal.aborted && error.name !== 'AbortError') loadError.value = error.message
    } finally {
      if (request === controller) {
        request = null
        if (!controller.signal.aborted) loading.value = false
      }
    }
    return false
  }

  function loadPage(nextCursors) {
    cursors.value = nextCursors
    return loadWorkspace({ foreground: true })
  }

  function restoreWorkspaceFilters(state) {
    keyword.value = state?.keyword || ''
    stage.value = state?.stage || ''
    stageLabel.value = state?.stageLabel || ''
    return loadPage(state?.cursors || [''])
  }

  function changeStage(event) {
    stage.value = event.target.value
    stageLabel.value = event.target.selectedOptions[0]?.textContent || ''
    return loadPage([''])
  }

  function changeKeyword(event) {
    keyword.value = event.target.value
    cursors.value = ['']
    cancelWorkspaceLoad()
    loading.value = true
    searchTimer = setTimeout(() => loadPage(['']), 250)
  }

  return { page, rows, counts, loading, loadError, keyword, stage, stageLabel, cursors, stages,
    loadWorkspace, loadPage, restoreWorkspaceFilters, changeStage, changeKeyword, cancelWorkspaceLoad }
}
