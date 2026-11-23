import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'
import { retailDemo } from './demo/retailServer.js'

const demoDatabase = fileURLToPath(new URL('./var/retail-demo.sqlite', import.meta.url))

export default defineConfig(({ command, isPreview, mode }) => ({
  plugins: [vue(), ...(mode === 'retail-demo' || (command === 'serve' && !isPreview) ? [retailDemo({ database: demoDatabase })] : [])],
  server: {
    watch: { ignored: ['**/var/retail-demo.sqlite*'] },
    proxy: { '/api': process.env.PARTNERLEDGER_API_TARGET || 'http://127.0.0.1:8080' },
  },
}))
