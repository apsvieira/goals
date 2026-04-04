import { defineConfig, type Plugin } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { readFileSync, writeFileSync } from 'fs'
import { resolve } from 'path'
import { fileURLToPath } from 'url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

/**
 * Vite plugin that replaces __BUILD_VERSION__ in sw.js after it is copied
 * from public/ to dist/. Files in public/ are copied as-is by Vite, so we
 * post-process the output in the closeBundle hook.
 */
function swVersionPlugin(): Plugin {
  return {
    name: 'sw-version',
    apply: 'build',
    closeBundle() {
      const swPath = resolve(__dirname, 'dist', 'sw.js')
      try {
        const content = readFileSync(swPath, 'utf-8')
        const now = new Date()
        const version = now.toISOString().replace(/[-:T]/g, '').slice(0, 14) // YYYYMMDDHHMMSS
        const updated = content.replace(/__BUILD_VERSION__/g, version)
        writeFileSync(swPath, updated, 'utf-8')
        console.log(`[sw-version] Injected CACHE_VERSION = '${version}' into dist/sw.js`)
      } catch (e) {
        console.warn('[sw-version] Could not patch sw.js:', e)
      }
    }
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte(), swVersionPlugin()],
  base: './', // Relative asset paths for Capacitor
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      }
    }
  }
})
