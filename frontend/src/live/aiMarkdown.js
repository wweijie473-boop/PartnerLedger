import { ref } from 'vue'

const THEME_STORAGE_KEY = 'partnerledger-ai-theme'

export function useAiTheme(storage = globalThis.window?.localStorage) {
  let initial = 'dark'
  try {
    const saved = storage?.getItem(THEME_STORAGE_KEY)
    if (saved === 'light' || saved === 'dark') initial = saved
  } catch {}

  const theme = ref(initial)

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    try {
      storage?.setItem(THEME_STORAGE_KEY, theme.value)
    } catch {}
  }

  return { theme, toggleTheme }
}

function formatInlineMd(escapedLine) {
  return escapedLine
    .replace(/\*\*([^*\n]+)\*\*/g, '<strong>$1</strong>')
    .replace(/`([^`\n]+)`/g, '<code class="ai-md-code">$1</code>')
}

export function formatBubbleHtml(raw) {
  const escaped = String(raw ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

  const lines = escaped.split('\n')
  const blocks = []
  let para = []
  let i = 0

  const flushPara = () => {
    if (!para.length) return
    blocks.push(`<p class="ai-md-p">${para.join('<br />')}</p>`)
    para = []
  }

  while (i < lines.length) {
    const line = lines[i]
    const trimmed = line.trim()

    if (!trimmed) {
      flushPara()
      i++
      continue
    }

    if (/^```/.test(trimmed)) {
      flushPara()
      i++
      const codeLines = []
      while (i < lines.length && !/^```/.test(lines[i].trim())) {
        codeLines.push(lines[i])
        i++
      }
      if (i < lines.length) i++
      blocks.push(`<pre class="ai-md-pre"><code>${codeLines.join('\n')}</code></pre>`)
      continue
    }

    if (
      i + 1 < lines.length &&
      /^\s*\|.+\|\s*$/.test(line) &&
      /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1])
    ) {
      flushPara()
      const parseRow = row => row.trim().replace(/^\||\|$/g, '').split('|').map(cell => formatInlineMd(cell.trim()))
      const headers = parseRow(line)
      i += 2
      const bodyRows = []
      while (i < lines.length && /^\s*\|.+\|\s*$/.test(lines[i])) {
        bodyRows.push(parseRow(lines[i]))
        i++
      }
      const thead = '<thead><tr>' + headers.map(h => `<th>${h}</th>`).join('') + '</tr></thead>'
      const tbody = bodyRows.length
        ? '<tbody>' + bodyRows.map(r => '<tr>' + r.map(c => `<td>${c}</td>`).join('') + '</tr>').join('') + '</tbody>'
        : ''
      blocks.push(`<div class="ai-md-table-wrap"><table class="ai-md-table">${thead}${tbody}</table></div>`)
      continue
    }

    if (/^(?:-{3,}|\*{3,})$/.test(trimmed)) {
      flushPara()
      blocks.push('<hr class="ai-md-hr" />')
      i++
      continue
    }

    const heading = /^(#{1,4})\s+(.+)$/.exec(trimmed)
    if (heading) {
      flushPara()
      const level = heading[1].length
      const cls = level <= 2 ? 'ai-md-h' : 'ai-md-subh'
      blocks.push(`<div class="${cls}">${formatInlineMd(heading[2])}</div>`)
      i++
      continue
    }

    if (/^[-*•]\s+/.test(trimmed)) {
      flushPara()
      const items = []
      while (i < lines.length && /^[-*•]\s+/.test(lines[i].trim())) {
        items.push(`<li>${formatInlineMd(lines[i].trim().replace(/^[-*•]\s+/, ''))}</li>`)
        i++
      }
      blocks.push(`<ul class="ai-md-list">${items.join('')}</ul>`)
      continue
    }

    if (/^\d+[.)、]\s+/.test(trimmed)) {
      flushPara()
      const items = []
      while (i < lines.length && /^\d+[.)、]\s+/.test(lines[i].trim())) {
        items.push(`<li>${formatInlineMd(lines[i].trim().replace(/^\d+[.)、]\s+/, ''))}</li>`)
        i++
      }
      blocks.push(`<ol class="ai-md-list is-ordered">${items.join('')}</ol>`)
      continue
    }

    para.push(formatInlineMd(line))
    i++
  }

  flushPara()
  return blocks.join('')
}
