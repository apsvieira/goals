import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Config for testing against PRODUCTION backend
// Run with: npm run dev -- --config vite.config.prod-test.ts
export default defineConfig({
  plugins: [svelte()],
  base: './',
  server: {
    proxy: {
      '/api': {
        target: 'https://goal-tracker-app.fly.dev',
        changeOrigin: true,
        secure: true,
      }
    }
  }
})
