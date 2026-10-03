import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { Hero } from './Hero'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters[2]!
const noop = () => undefined

function hero(pack: Partial<typeof chapter.pack>) {
  return render(
    <Hero
      chapter={{ ...chapter, pack: { ...chapter.pack, ...pack } }}
      onOpenWiki={noop}
      onOpenSettings={noop}
    />,
  )
}

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
    // The chapter's state is still there.
    expect(screen.getByText(/Released|In development|Planned/)).toBeInTheDocument()
  })

  it('shows the half that is settled', () => {
    hero({ loader: '[PLACEHOLDER]', minecraft: '1.20.6', mods: 31 })
    expect(screen.getByText('1.20.6')).toBeInTheDocument()
    expect(screen.getByText('31 mods')).toBeInTheDocument()
  })
})
