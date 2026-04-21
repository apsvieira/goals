import { defineConfig, type Plugin } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { sentryVitePlugin } from '@sentry/vite-plugin'
import { readFileSync, writeFileSync } from 'fs'
import { resolve } from 'path'
import { fileURLToPath } from 'url'

/**
 * Post-build cleanup: delete `.map` files from dist/ AFTER `sentryVitePlugin`
 * has finished its upload pass. We keep sourcemaps as 'hidden' (no
 * `sourceMappingURL` comment) so Sentry can still read them during the
 * upload, then strip the map files from the build output here so nothing
 * ships to the device. `enforce: 'post'` + position after `sentryVitePlugin`
 * in the plugins array together guarantee execution order for `closeBundle`.
 */
function stripMapsAfterUpload(): Plugin {
  return {
    name: 'strip-source-maps-after-upload',
    apply: 'build',
    enforce: 'post',
    async closeBundle() {
      const { readdir, unlink } = await import('node:fs/promises')
      const { join } = await import('node:path')
      const walk = async (dir: string): Promise<void> => {
        let entries: Awaited<ReturnType<typeof readdir>>
        try {
          entries = await readdir(dir, { withFileTypes: true })
        } catch {
          return
        }
        for (const e of entries) {
          const p = join(dir, e.name)
          if (e.isDirectory()) await walk(p)
          else if (e.name.endsWith('.map')) await unlink(p)
        }
      }
      await walk('dist')
    },
  }
}

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

/**
 * Conditionally enable the Sentry vite plugin for source-map upload.
 * Activates ONLY in production builds AND only when all three required
 * secrets are present. Missing any → plugin is skipped silently (one dev-
 * mode info line) so local and PR builds without Sentry credentials keep
 * succeeding.
 */
function maybeSentryPlugin(isProd: boolean): Plugin | undefined {
  if (!isProd) return undefined
  const authToken = process.env.SENTRY_AUTH_TOKEN
  const org = process.env.SENTRY_ORG
  const project = process.env.SENTRY_PROJECT
  if (!authToken || !org || !project) {
    console.info(
      '[sentry-vite-plugin] skipping source-map upload (missing SENTRY_AUTH_TOKEN / SENTRY_ORG / SENTRY_PROJECT)',
    )
    return undefined
  }
  return sentryVitePlugin({
    authToken,
    org,
    project,
    // Only upload; don't modify unrelated build behavior.
    telemetry: false,
  }) as Plugin
}

// https://vite.dev/config/
export default defineConfig(({ command }) => {
  const isProd = command === 'build'
  const sentry = maybeSentryPlugin(isProd)
  return {
    plugins: [
      svelte(),
      swVersionPlugin(),
      ...(sentry ? [sentry] : []),
      // MUST come after sentryVitePlugin so the cleanup runs after the upload
      // pass (closeBundle order = plugin order, adjusted by enforce).
      stripMapsAfterUpload(),
    ],
    base: './', // Relative asset paths for Capacitor
    // 'hidden' keeps `.map` files on disk (so @sentry/vite-plugin can upload
    // them) but omits the `//# sourceMappingURL=` comment from emitted JS —
    // and the `stripMapsAfterUpload` plugin then deletes the map files from
    // dist/ so nothing ships to device.
    build: {
      sourcemap: 'hidden',
    },
    server: {
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  }
})
