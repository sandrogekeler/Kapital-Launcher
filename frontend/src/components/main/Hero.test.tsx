import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Hero } from './Hero'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters[2]!
const noop = () => undefined

function hero(pack: Partial<typeof chapter.pack>, version: string | null = '1.0.1') {
  return render(
    <Hero
      chapter={{ ...chapter, pack: { ...chapter.pack, ...pack } }}
      onOpenWiki={noop}
      onOpenSettings={noop}
      onOpenLogs={noop}
      version={version}
    />,
  )
}

describe('Hero tools', () => {
  afterEach(cleanup)

  it('has the settings pen, the logs console and the wiki book, each doing its own thing', () => {
    const onOpenWiki = vi.fn()
    const onOpenSettings = vi.fn()
    const onOpenLogs = vi.fn()
    render(
      <Hero
        chapter={chapter}
        onOpenWiki={onOpenWiki}
        onOpenSettings={onOpenSettings}
        onOpenLogs={onOpenLogs}
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
    expect([onOpenWiki, onOpenSettings, onOpenLogs].map((f) => f.mock.calls.length)).toEqual([
      0, 0, 1,
    ])
    fireEvent.click(screen.getByRole('button', { name: `${chapter.name} settings` }))
    fireEvent.click(screen.getByRole('button', { name: 'Read the history on the wiki' }))
    expect([onOpenWiki, onOpenSettings, onOpenLogs].map((f) => f.mock.calls.length)).toEqual([
      1, 1, 1,
    ])
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
