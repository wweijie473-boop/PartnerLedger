import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { retailDemo } from './demo/retailServer.js'

export default defineConfig(({ mode }) => ({
  plugins: [vue(), ...(mode === 'retail-demo' ? [retailDemo()] : [])],
  server: {
    proxy: { '/api': process.env.PARTNERLEDGER_API_TARGET || 'http://127.0.0.1:8080' },
  },
}))
