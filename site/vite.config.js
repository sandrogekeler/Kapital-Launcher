import { readFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

// The page's three links, filled in at build time so the built HTML carries no
// script. Download and GitHub come from links.json and stay null until there is
// something public to point at: a null Download renders its "coming soon"
// fallback, a null GitHub renders nothing. The wiki is the manifest's own
// baseUrl, so the app and the site cannot disagree about where it lives.
function readLinks() {
  const links = JSON.parse(readFileSync(new URL('./links.json', import.meta.url), 'utf8'))
  const manifest = JSON.parse(
    readFileSync(new URL('../data/launcher.json', import.meta.url), 'utf8'),
  )
  const all = { download: links.download, github: links.github, wiki: manifest.wiki.baseUrl }
  for (const [key, url] of Object.entries(all)) {
    if (url === null) continue
    if (typeof url !== 'string' || new URL(url).protocol !== 'https:') {
      throw new Error(`site: ${key} link must be an https URL or null, got ${JSON.stringify(url)}`)
    }
  }
  if (all.wiki === null) throw new Error('site: the manifest has no wiki.baseUrl')
  return all
}

const escapeAttr = (s) => s.replaceAll('&', '&amp;').replaceAll('"', '&quot;')

function fillLinks(html, links) {
  const seen = new Set()
  const out = html
    .replace(/<a\s+data-link="(\w+)"([^>]*)>([\s\S]*?)<\/a\s*>/g, (_, key, attrs, body) => {
      if (!(key in links)) throw new Error(`site: index.html links "${key}", links.json has no such key`)
      seen.add(key)
      const url = links[key]
      return url ? `<a href="${escapeAttr(url)}"${attrs}>${body}</a>` : ''
    })
    .replace(/<span\s+data-link-fallback="(\w+)"([^>]*)>([\s\S]*?)<\/span\s*>/g, (_, key, attrs, body) =>
      links[key] ? '' : `<span${attrs}>${body}</span>`,
    )
  for (const key of Object.keys(links)) {
    if (!seen.has(key)) throw new Error(`site: index.html has no data-link="${key}"`)
  }
  return out
}

function links() {
  const resolved = readLinks()
  return {
    name: 'kapital:links',
    transformIndexHtml: { order: 'pre', handler: (html) => fillLinks(html, resolved) },
  }
}

export default defineConfig({
  plugins: [tailwindcss(), links()],
  build: {
    // Every asset is a file, never a data: URI, so the CSP in public/_headers
    // can stay at img-src 'self' and font-src 'self'.
    assetsInlineLimit: 0,
  },
})
