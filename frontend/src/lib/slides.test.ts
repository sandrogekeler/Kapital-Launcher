import { describe, expect, it } from 'vitest'
import type { WikiPage, WikiShot } from '../types'
import { BUNDLED_MANIFEST } from './manifest'
import { pickSlide, postsForShot, shotsForChapter } from './slides'

const luxemburg = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'luxemburg')!
const frangfurd = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!

const page = (id: string, eras: string[], related: string[] = []): WikiPage => ({
  title: id,
  line: 'A line.',
  url: `https://wiki.example/wiki/${id}`,
  eras,
  id,
  related,
})
const shot = (src: string, era: string, subject = ''): WikiShot => ({ era, src, subject })

const castle = page('locations/Bellum Castle.md', ['Luxemburg'], ['factions/The Avari.md'])
const avari = page(
  'factions/The Avari.md',
  ['Luxemburg'],
  ['locations/Bellum Castle.md', 'locations/Ravana Ruins.md'],
)
const ruins = page(
  'locations/Ravana Ruins.md',
  ['Luxemburg', 'Frangfurd'],
  ['factions/The Avari.md'],
)
const okinowa = page('locations/Okinowa.md', ['Luxemburg'])
const maiden = page('vehicles/The Copper Maiden.md', ['Frangfurd'])
const pages = [castle, avari, ruins, okinowa, maiden]

describe('slides', () => {
  it('keeps the screenshots of the chapter era', () => {
    const shots = [shot('/a', 'Luxemburg'), shot('/b', 'Frangfurd')]
    expect(shotsForChapter(shots, luxemburg).map((s) => s.src)).toEqual(['/a'])
  })

  it('pairs a picture with its subject or a page related to it, of the era', () => {
    const lux = pages.filter((p) => p.eras.includes('Luxemburg'))
    const posts = postsForShot(shot('/c', 'Luxemburg', castle.id), pages, lux)
    expect(posts.map((p) => p.id)).toEqual([castle.id, avari.id])
    // The Avari relate to the ruins too, but only Frangfurd pages may follow a Frangfurd picture.
    const fr = pages.filter((p) => p.eras.includes('Frangfurd'))
    expect(postsForShot(shot('/r', 'Frangfurd', avari.id), pages, fr).map((p) => p.id)).toEqual([
      ruins.id,
    ])
  })

  it('takes any page of the era for a picture with no subject or none linked', () => {
    const lux = pages.filter((p) => p.eras.includes('Luxemburg'))
    expect(postsForShot(shot('/x', 'Luxemburg'), pages, lux)).toHaveLength(lux.length)
    expect(postsForShot(shot('/y', 'Luxemburg', 'locations/Gone.md'), pages, lux)).toHaveLength(
      lux.length,
    )
    expect(postsForShot(undefined, pages, lux)).toHaveLength(lux.length)
  })

  it('never shows the picture or the post just shown when there is a choice', () => {
    const shots = [shot('/1', 'Luxemburg', castle.id), shot('/2', 'Luxemburg', castle.id)]
    const first = pickSlide(shots, pages, luxemburg, undefined, () => 0)
    expect(first.art).toBe('/1')
    expect(first.page?.id).toBe(castle.id)
    const next = pickSlide(shots, pages, luxemburg, first, () => 0)
    expect(next.art).toBe('/2')
    expect(next.page?.id).toBe(avari.id)
  })

  it('has no picture without screenshots, and keeps the one there is', () => {
    const none = pickSlide([], pages, frangfurd, undefined, () => 0)
    expect(none.art).toBeUndefined()
    expect(none.page?.eras).toContain('Frangfurd')
    const one = [shot('/only', 'Frangfurd', maiden.id)]
    const first = pickSlide(one, pages, frangfurd, undefined)
    expect(pickSlide(one, pages, frangfurd, first).art).toBe('/only')
  })

  it('has no post without pages, which shows the teaser', () => {
    expect(pickSlide([shot('/1', 'Luxemburg')], [], luxemburg, undefined).page).toBeUndefined()
  })
})
