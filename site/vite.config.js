import { readFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

// The page's links, filled in at build time so the built HTML carries no inline
// script. GitHub and `release` come from links.json and stay null until there
// is something public to point at: a null GitHub renders nothing, a null
// release renders the "Download · coming soon" fallback and leaves out the
// download popup. A release is a tag, and the two installer URLs and the
// release notes URL are derived from it and the GitHub link, so nothing is
// written twice. The wiki is the manifest's own baseUrl, so the app and the
// site cannot disagree about where it lives.
const TAG = /^v\d+\.\d+\.\d+(-(alpha|beta)\.\d+)?$/

function readLinks() {
  const links = JSON.parse(readFileSync(new URL('./links.json', import.meta.url), 'utf8'))
  const manifest = JSON.parse(
    readFileSync(new URL('../data/launcher.json', import.meta.url), 'utf8'),
  )
  const all = { github: links.github, wiki: manifest.wiki.baseUrl }
  for (const [key, url] of Object.entries(all)) {
    if (url === null) continue
    if (typeof url !== 'string' || new URL(url).protocol !== 'https:') {
      throw new Error(`site: ${key} link must be an https URL or null, got ${JSON.stringify(url)}`)
    }
  }
  if (all.wiki === null) throw new Error('site: the manifest has no wiki.baseUrl')

  const tag = links.release
  let release = null
  if (tag !== null) {
    if (typeof tag !== 'string' || !TAG.test(tag)) {
      throw new Error(
        `site: release must be a tag like v0.1.0-beta.1 or null, got ${JSON.stringify(tag)}`,
      )
    }
    if (all.github === null) throw new Error('site: a release needs the github link')
    const base = all.github.replace(/\/+$/, '')
    release = {
      tag,
      WINDOWS_URL: `${base}/releases/download/${tag}/Kapital-Launcher-${tag}-windows-amd64-setup.exe`,
      MACOS_URL: `${base}/releases/download/${tag}/Kapital-Launcher-${tag}-macos-universal.dmg`,
      NOTES_URL: `${base}/releases/tag/${tag}`,
    }
  }
  return { ...all, release }
}

const escapeAttr = (s) => s.replaceAll('&', '&amp;').replaceAll('"', '&quot;')

// The popup and its button sit between release markers in index.html: kept,
// with their placeholders filled, when a release is set, dropped when not.
const BLOCK = /[ \t]*<!--release:start-->[\s\S]*?<!--release:end-->[ \t]*\r?\n?/g

function fillRelease(html, release) {
  if (release === null) return html.replace(BLOCK, '')
  const out = html
    .replaceAll('<!--release:start-->', '')
    .replaceAll('<!--release:end-->', '')
    .replace(/%(WINDOWS_URL|MACOS_URL|NOTES_URL)%/g, (_, key) => escapeAttr(release[key]))
  if (/%[A-Z_]+_URL%/.test(out)) throw new Error('site: an unfilled release placeholder is left')
  return out
}

function fillLinks(html, links) {
  const seen = new Set()
  const out = fillRelease(html, links.release)
    .replace(/<a\s+data-link="(\w+)"([^>]*)>([\s\S]*?)<\/a\s*>/g, (_, key, attrs, body) => {
      if (key === 'release' || !(key in links)) {
        throw new Error(`site: index.html links "${key}", links.json has no such key`)
      }
      seen.add(key)
      const url = links[key]
      return url ? `<a href="${escapeAttr(url)}"${attrs}>${body}</a>` : ''
    })
    .replace(/<span\s+data-link-fallback="(\w+)"([^>]*)>([\s\S]*?)<\/span\s*>/g, (_, key, attrs, body) =>
      links[key] ? '' : `<span${attrs}>${body}</span>`,
    )
  for (const key of ['github', 'wiki']) {
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
    // can stay at img-src 'self' and font-src 'self'. The script is a file too,
    // never inline, so script-src 'self' is all it needs.
    assetsInlineLimit: 0,
  },
})
