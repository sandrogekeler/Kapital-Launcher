/// <reference types="vitest/config" />
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

export default defineConfig({
  plugins: [react(), tailwindcss(), stripCspInDev()],
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
