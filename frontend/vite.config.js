import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

const demoDatabase = fileURLToPath(new URL('./var/retail-demo.sqlite', import.meta.url))

export default defineConfig(async ({ command, isPreview, mode }) => {
  const plugins = [vue()]
  if (mode === 'retail-demo' || (command === 'serve' && !isPreview)) {
    const { retailDemo } = await import('./demo/retailServer.js')
    plugins.push(retailDemo({ database: demoDatabase }))
  }
  return {
    plugins,
    server: {
      watch: { ignored: ['**/var/retail-demo.sqlite*'] },
      proxy: { '/api': process.env.PARTNERLEDGER_API_TARGET || 'http://127.0.0.1:8080' },
    },
  }
})
