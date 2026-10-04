/// <reference types="vitest/config" />
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { defineConfig } from 'vite'
import type { Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// index.html, and splash.html for the loading card's window (#97), carry the
// app's Content-Security-Policy and the built output ships them verbatim. The
// dev server cannot: the React refresh preamble is an inline script and Vite's
// HMR client speaks over a websocket, both of which `script-src 'self'`
// refuses. The tag is removed in serve mode only, so each page stays the one
// place its policy is written and every build carries it. The cost is that dev
// never runs under the policy.
function stripCspInDev(): Plugin {
  return {
    name: 'kapital:strip-csp-in-dev',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      handler: (html) =>
        html.replace(/<meta[^>]*http-equiv="Content-Security-Policy"[^>]*>\s*/i, ''),
    },
  }
}

// A chapter's map is framed in the launcher (issue 161), and `frame-src 'none'`
// in index.html would refuse it. The build writes into that directive exactly
// the map origins the bundled manifest names (scheme://host:port, nothing
// wider), or leaves 'none' when it names none. The shape is the one the Go
// loader enforces for a map address (services.checkMapURL: http or https on a
// playit tunnel, *.tun.ply.gg, with a port), and a manifest that breaks it
// fails the build rather than loosening the policy. The loading card's page
// (splash.html) is not touched: it frames nothing. scripts/check-csp.mjs reads
// the built output back and holds it to the manifest.
const MAP_URL =
  /^(https?):\/\/((?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+tun\.ply\.gg):([1-9][0-9]{0,4})\/?$/

function mapOrigins(): string[] {
  const manifest = JSON.parse(
    readFileSync(path.resolve(import.meta.dirname, '../data/launcher.json'), 'utf8'),
  ) as { chapters: { id: string; map?: string | null }[] }
  const origins = new Set<string>()
  for (const chapter of manifest.chapters) {
    if (chapter.map == null) continue
    const m = MAP_URL.exec(chapter.map)
    if (!m || Number(m[3]) > 65535) {
      throw new Error(`data/launcher.json: ${chapter.id}'s map is not a playit tunnel with a port`)
    }
    origins.add(`${m[1]}://${m[2]}:${m[3]}`)
  }
  return [...origins].sort()
}

function mapFrameSrc(): Plugin {
  return {
    name: 'kapital:map-frame-src',
    apply: 'build',
    transformIndexHtml: {
      order: 'pre',
      handler: (html, ctx) => {
        if (path.basename(ctx.filename) !== 'index.html') return html
        const none = "frame-src 'none'"
        if (!html.includes(none)) throw new Error(`index.html has no ${none} to fill`)
        const origins = mapOrigins()
        return html.replace(none, `frame-src ${origins.length ? origins.join(' ') : "'none'"}`)
      },
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), stripCspInDev(), mapFrameSrc()],
  build: {
    // Two pages: the launcher, and the loading card's window (#97), whose
    // entry chunk is its own beside the launcher's.
    rollupOptions: { input: { index: 'index.html', splash: 'splash.html' } },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test-setup.ts'],
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: [
        'src/**/*.test.{ts,tsx}',
        'src/styles/tokens.ts',
        'src/main.tsx',
        'src/test-css.ts',
        'src/splash/main.tsx',
        'src/**/*.d.ts',
      ],
      reporter: ['text', 'text-summary'],
      // The floor. A ratchet: raised as coverage rises, never lowered to make a
      // build pass. Measured 90.8% on 19 tests, 2026-09-28.
      thresholds: { lines: 80 },
    },
  },
})
