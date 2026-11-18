import { createApp } from 'vue'
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-dialog.css'
import 'element-plus/theme-chalk/el-overlay.css'
import App from './App.vue'
import UiGallery from './UiGallery.vue'
import './ui/theme.css'
import './style.css'

const isGallery = window.location.pathname.replace(/\/$/, '') === '/ui'
if (isGallery) document.title = 'UI 组件库 · PartnerLedger'
createApp(isGallery ? UiGallery : App).mount('#app')
