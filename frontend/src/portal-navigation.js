import { entrances } from './entrances.js'
import { workspacePages } from './workspaceNavigation.js'

export const portalMenus = [
  {
    id: 'solutions', label: '解决方案', links: [
      { label: '商机拾遗', href: '/experience/dealer?view=opportunities', icon: 'workflow', description: '记下客户的额外需求，找做过类似项目的伙伴', keywords: '商机 伙伴 联系 条件 介绍费' },
      { label: '拜访记录', href: '/experience/dealer?view=visits', icon: 'request', description: '记下这次聊过的需求和下次需要确认的事', keywords: '记拜访 草稿 走访' },
      { label: '表格工作台', href: '/tables', icon: 'catalog', description: '在表格里整理项目资料、查找和替换内容', keywords: '表格 工作表 Excel 数据 搜索' },
    ],
  },
  {
    id: 'collaboration', label: '协作', links: [
      { label: '三方协作流程', href: '#journey', icon: 'workflow', description: '用一个 ERP 项目看看三方怎么从询价走到验收', keywords: '采购 ERP 报价 下单 订单 交付 验收 需求与询价' },
      ...entrances.map(({ role, label, icon, description }) => ({
        label: label + '演示', href: '/experience/' + role, icon, description, keywords: '工作台 本地演示',
      })),
    ],
  },
]

export const portalResources = [
  { label: '了解我们', href: '#about', icon: 'info', description: '看看渠智链能帮软件采购团队做什么', keywords: '关于我们 PartnerLedger 官网 平台 介绍' },
  { label: '帮助', href: '#help', icon: 'info', description: '查找登录、注册和免登录演示入口', keywords: '登录 注册 指南 支持 使用帮助 新手' },
  { label: '界面组件', href: '/ui', icon: 'catalog', description: '查看界面组件、操作反馈和状态示例', keywords: '设计 组件 UI 界面' },
]

const entries = [...portalMenus.flatMap(menu => menu.links), ...portalResources]
const normalize = value => value.normalize('NFKC').trim().toLocaleLowerCase('zh-CN')

export function searchPortal(query, role) {
  const terms = normalize(query).split(/\s+/).filter(Boolean)
  if (!terms.length) return entries.slice(0, 4)
  const workspaces = entrances.filter(item => !role || role === item.role)
    .flatMap(item => workspacePages(item.role).filter(page => !page.direct).map(page => ({
      label: page.label,
      href: '/login/' + item.role + '?' + new URLSearchParams({ view: page.id }),
      icon: page.icon,
      description: item.label + '工作台',
      keywords: '',
    })))
  const searchable = role ? [...workspaces, ...entries] : [...entries, ...workspaces]
  return searchable.map(entry => {
    const title = normalize(entry.label)
    const content = normalize([entry.label, entry.description, entry.keywords].join(' '))
    const score = terms.every(term => content.includes(term))
      ? terms.reduce((total, term) => total + (title.includes(term) ? 2 : 1), 0)
      : 0
    return { entry, score }
  }).filter(item => item.score > 0)
    .sort((a, b) => b.score - a.score)
    .slice(0, 6)
    .map(item => item.entry)
}
