import { describe, expect, it } from 'vitest'
import type { WikiPage } from '../types'
import { BUNDLED_MANIFEST } from './manifest'
import { pagesForChapter, pickPage } from './wiki'

const page = (url: string, ...eras: string[]): WikiPage => ({
  title: url,
  line: 'A line.',
  url: `https://wiki.example${url}`,
  eras,
  id: url,
  related: [],
})
const pages = [
  page('/wiki/a', 'Frangfurd'),
  page('/wiki/b', 'Frangfurd'),
  page('/wiki/c', 'Luxemburg', 'Lichdenstein'),
]
const [frangfurd, luxemburg, lichdenstein] = [
  BUNDLED_MANIFEST.chapters[2]!,
  BUNDLED_MANIFEST.chapters[0]!,
  BUNDLED_MANIFEST.chapters[1]!,
]

describe('wiki picking', () => {
  it('matches pages to a chapter by era name, multi-era pages to each', () => {
    expect(pagesForChapter(pages, frangfurd).map((p) => p.title)).toEqual(['/wiki/a', '/wiki/b'])
    expect(pagesForChapter(pages, luxemburg).map((p) => p.title)).toEqual(['/wiki/c'])
    expect(pagesForChapter(pages, lichdenstein).map((p) => p.title)).toEqual(['/wiki/c'])
  })

  it('picks by the random draw and skips the page just shown', () => {
    const two = pagesForChapter(pages, frangfurd)
    expect(pickPage(two, undefined, () => 0)).toBe(two[0])
    expect(pickPage(two, undefined, () => 0.99)).toBe(two[1])
    expect(pickPage(two, two[0], () => 0)).toBe(two[1])
    expect(pickPage(two, two[1], () => 0.99)).toBe(two[0])
  })

  it('shows the only page again rather than nothing, and nothing when there is none', () => {
    const one = pagesForChapter(pages, luxemburg)
    expect(pickPage(one, one[0], () => 0.5)).toBe(one[0])
    expect(pickPage([], undefined)).toBeUndefined()
  })
})
