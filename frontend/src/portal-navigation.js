import { entrances } from './entrances.js'

export const portalMenus = [
  {
    id: 'solutions', label: '解决方案', links: [
      { label: '商机拾遗', href: '/experience/dealer?view=opportunities', icon: 'workflow', description: '记录附加需求，匹配伙伴并跟进引荐', keywords: '商机 伙伴 联系 条件 介绍费' },
      { label: '拜访记录', href: '/experience/dealer?view=visits', icon: 'request', description: '整理走访记录和客户的附加需求', keywords: '记拜访 草稿 走访' },
      { label: '表格工作台', href: '/tables', icon: 'catalog', description: '用表格整理数据，支持查找替换', keywords: '表格 工作表 Excel 数据 搜索' },
    ],
  },
  {
    id: 'collaboration', label: '协作', links: [
      { label: '三方协作流程', href: '#journey', icon: 'workflow', description: '查看从需求、询价到交付验收的流程', keywords: '采购 ERP 报价 下单 订单 交付 验收 需求与询价' },
      ...entrances.map(({ role, label, icon, description }) => ({
        label: label + '演示', href: '/experience/' + role, icon, description, keywords: '工作台 本地演示',
      })),
    ],
  },
]

export const portalResources = [
  { label: '了解我们', href: '#about', icon: 'info', description: '认识渠智链与三方协作的业务范围', keywords: '关于我们 PartnerLedger 官网 平台 介绍' },
  { label: '帮助', href: '#help', icon: 'info', description: '了解如何选择登录身份和开始体验', keywords: '登录 指南 支持 使用帮助 新手' },
  { label: '界面组件', href: '/ui', icon: 'catalog', description: '查看界面组件、操作反馈和状态示例', keywords: '设计 组件 UI 界面' },
]

const entries = [...portalMenus.flatMap(menu => menu.links), ...portalResources]
const normalize = value => value.normalize('NFKC').trim().toLocaleLowerCase('zh-CN')

export function searchPortal(query) {
  const terms = normalize(query).split(/\s+/).filter(Boolean)
  if (!terms.length) return entries.slice(0, 4)
  return entries.map(entry => {
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
