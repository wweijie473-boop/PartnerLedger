import { createApp, h } from 'vue'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-dialog.css'
import 'element-plus/theme-chalk/el-overlay.css'
import './ui/theme.css'
import './style.css'
import { entrances } from './entrances.js'
import { reloadOnSessionChange } from './auth-session.js'

window.addEventListener('pageshow', reloadOnSessionChange)

const path = window.location.pathname.replace(/\/+$/, '') || '/'
const pages = {
  '/': ['软件渠道协作平台', () => import('./Portal.vue')],
  '/ui': ['UI 组件库', () => import('./UiGallery.vue')],
  '/demo': ['双角色原型', () => import('./App.vue')],
  '/tables': ['表格工作台', () => import('./tables/TableStudio.vue')],
  '/login': ['登录', () => import('./LiveApp.vue')],
  '/register': ['注册', () => import('./LiveApp.vue'), { mode: 'register' }],
}
for (const { role, label } of entrances) {
  pages['/login/' + role] = [label + '登录', () => import('./LiveApp.vue'), { role }]
  pages['/register/' + role] = [label + '注册', () => import('./LiveApp.vue'), { role, mode: 'register' }]
  pages['/experience/' + role] = [label + '工作台 · 本地演示', () => import('./channel/Workspace.vue'), { role }]
}
const route = pages[path]
if (route) {
  const [title, load, props] = route
  document.title = title + ' · 渠智链'
  load().then(({ default: component }) => createApp(component, props).mount('#app')).catch(() => {
    document.getElementById('app').textContent = '页面加载失败，请刷新重试。'
  })
} else {
  document.title = '页面不存在 · 渠智链'
  createApp({ render: () => h('main', { class: 'content' }, [h('h1', '页面不存在'), h('p', '请从平台首页选择工作台入口。'), h('a', { href: '/' }, '返回首页')]) }).mount('#app')
}
