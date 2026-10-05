import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Hero } from './Hero'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters[2]!
const noop = () => undefined

function hero(pack: Partial<typeof chapter.pack>, version: string | null = '1.0.1') {
  return render(
    <Hero
      chapter={{ ...chapter, pack: { ...chapter.pack, ...pack } }}
      onOpenSettings={noop}
      onOpenLogs={noop}
      onOpenMap={noop}
      version={version}
    />,
  )
}

describe('Hero tools', () => {
  afterEach(cleanup)

  it('has the settings pen, the logs console and the map, each doing its own thing', () => {
    const onOpenMap = vi.fn()
    const onOpenSettings = vi.fn()
    const onOpenLogs = vi.fn()
    render(
      <Hero
        chapter={chapter}
        onOpenSettings={onOpenSettings}
        onOpenLogs={onOpenLogs}
        onOpenMap={onOpenMap}
        version="1.0.1"
      />,
    )
    // One group: square cells, the group's frame rounds the outer corners.
    const group = screen.getByRole('group', { name: `${chapter.name} tools` })
    expect(group).toHaveClass('overflow-hidden', 'rounded-md')
    expect(group).not.toHaveClass('gap-1')
    for (const button of Array.from(group.querySelectorAll('button'))) {
      expect(button).not.toHaveClass('rounded-md')
    }
    fireEvent.click(screen.getByRole('button', { name: `Logs for ${chapter.name}` }))
    expect([onOpenMap, onOpenSettings, onOpenLogs].map((f) => f.mock.calls.length)).toEqual([
      0, 0, 1,
    ])
    fireEvent.click(screen.getByRole('button', { name: `${chapter.name} settings` }))
    // The map takes the wiki book's place in the group (issue 161); the wiki
    // link lives in the panels below.
    fireEvent.click(screen.getByRole('button', { name: `Map of ${chapter.name}` }))
    expect([onOpenMap, onOpenSettings, onOpenLogs].map((f) => f.mock.calls.length)).toEqual([
      1, 1, 1,
    ])
    expect(screen.queryByRole('button', { name: 'Read the history on the wiki' })).toBeNull()
    expect(group.querySelectorAll('button')).toHaveLength(3)
  })
})

describe('Hero pills', () => {
  afterEach(cleanup)

  it('names the loader, the game version and the mods when they are settled', () => {
    hero({ loader: 'NeoForge', minecraft: '1.21.1', mods: 96 })
    expect(screen.getByText('NeoForge 1.21.1')).toBeInTheDocument()
    expect(screen.getByText('96 mods')).toBeInTheDocument()
  })

  it('shows no bracketed placeholder for what is not settled', () => {
    hero({ loader: '[PLACEHOLDER]', minecraft: '[PLACEHOLDER]', mods: null })
    expect(screen.queryByText(/\[/)).toBeNull()
    expect(screen.queryByText(/mods/)).toBeNull()
    // The pack's version is still there, in place of the old state pill.
    expect(screen.getByText('Version 1.0.1')).toBeInTheDocument()
    expect(screen.queryByText(/Released|In development|Planned/)).toBeNull()
  })

  it('leaves the version pill out when nobody knows the version yet', () => {
    hero({}, null)
    expect(screen.queryByText(/^Version/)).toBeNull()
    cleanup()
    hero({}, '[PLACEHOLDER]')
    expect(screen.queryByText(/^Version/)).toBeNull()
  })

  it('shows the half that is settled', () => {
    hero({ loader: '[PLACEHOLDER]', minecraft: '1.20.6', mods: 31 })
    expect(screen.getByText('1.20.6')).toBeInTheDocument()
    expect(screen.getByText('31 mods')).toBeInTheDocument()
  })
})

describe('Hero panorama (issue 195)', () => {
  afterEach(cleanup)
  const faces = Array.from({ length: 6 }, (_, n) => `/panorama/${chapter.id}/panorama_${n}.png`)
  const show = (panorama?: readonly string[]) =>
    render(
      <Hero
        chapter={chapter}
        panorama={panorama}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
        version="1.0.1"
      />,
    )

  it('says the panorama is loading until its faces are decoded, and draws no cube meanwhile', async () => {
    const real = HTMLImageElement.prototype.decode
    let release: () => void = () => undefined
    const decoded = new Promise<void>((resolve) => (release = resolve))
    HTMLImageElement.prototype.decode = () => decoded
    try {
      const other = faces.map((f) => `${f}?v=slow`)
      const { container } = show(other)
      expect(await screen.findByRole('status')).toHaveTextContent('Loading panorama…')
      expect(container.querySelector('.panorama')).toBeNull()
      await act(async () => release())
      await vi.waitFor(() => expect(container.querySelector('.panorama')).not.toBeNull())
      expect(screen.queryByText('Loading panorama…')).toBeNull()
    } finally {
      HTMLImageElement.prototype.decode = real
    }
  })

  it("draws the cube over the chapter's own picture and under the same scrim", async () => {
    const { container } = show(faces)
    await screen.findByText(chapter.blurb)
    await vi.waitFor(() => expect(container.querySelector('.panorama')).not.toBeNull())
    const section = container.querySelector('section')!
    const children = Array.from(section.children)
    const picture = children.findIndex((c) => c.querySelector('img[draggable=false]'))
    const cube = children.findIndex((c) => c.classList.contains('panorama'))
    const scrim = children.findIndex((c) => c.classList.contains('scrim-hero'))
    // The picture is what shows until the faces load, and the scrim keeps the text readable.
    expect(picture).toBeGreaterThanOrEqual(0)
    expect(cube).toBeGreaterThan(picture)
    expect(scrim).toBeGreaterThan(cube)
    expect(section.querySelectorAll('.panorama-face')).toHaveLength(6)
  })

  it('keeps only its own picture with no panorama, or with one that is not six faces', async () => {
    for (const panorama of [undefined, faces.slice(0, 5)]) {
      const { container, unmount } = show(panorama)
      await screen.findByText(chapter.blurb)
      expect(container.querySelector('.panorama')).toBeNull()
      expect(container.querySelector('.scrim-hero')).not.toBeNull()
      unmount()
    }
  })
})
