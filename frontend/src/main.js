import { createApp } from 'vue'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-dialog.css'
import 'element-plus/theme-chalk/el-overlay.css'
import App from './App.vue'
import LiveApp from './LiveApp.vue'
import UiGallery from './UiGallery.vue'
import './ui/theme.css'
import './style.css'

const path = window.location.pathname.replace(/\/$/, '')
const pages = {
  '/ui': { component: UiGallery, title: 'UI 组件库 · PartnerLedger' },
  '/demo': { component: App, title: '模拟演示 · PartnerLedger' },
}
const page = pages[path] ?? { component: LiveApp, title: '渠智链 AI · PartnerLedger' }
document.title = page.title
createApp(page.component).mount('#app')
