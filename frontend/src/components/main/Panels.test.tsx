import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { Panels } from './Panels'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters[2]!
const noop = () => undefined

function panels(over: Partial<ComponentProps<typeof Panels>> = {}) {
  return render(
    <Panels
      chapter={chapter}
      installed={true}
      sizeBytes={undefined}
      packState={undefined}
      changelog={undefined}
      wikiPage={undefined}
      onOpenWiki={noop}
      {...over}
    />,
  )
}

const row = (label: string) => screen.getByText(label).nextElementSibling as HTMLElement

describe('Panels', () => {
  afterEach(cleanup)

  it('says Not installed for the version and the size of a chapter that is not installed', () => {
    panels({ installed: false })
    expect(row('Version')).toHaveTextContent('Not installed')
    expect(row('Size')).toHaveTextContent('Not installed')
    expect(row('Version')).toHaveClass('text-fg-faint')
    expect(screen.queryByText(/PLACEHOLDER/)).toBeNull()
  })

  it('says Unknown, faint, for a fact that nobody has settled', () => {
    const open = { ...chapter, pack: { ...chapter.pack, mods: null, minecraft: '[PLACEHOLDER]' } }
    panels({ chapter: open, installed: true })
    expect(row('Mods')).toHaveTextContent('Unknown')
    expect(row('Minecraft')).toHaveTextContent('Unknown')
    expect(row('Size')).toHaveTextContent('Unknown')
    expect(row('Mods')).not.toHaveClass('font-mono')
  })

  it('keeps numbers and versions in the data face and words in the UI face', () => {
    panels({ sizeBytes: 1_500_000_000 })
    expect(row('Size')).toHaveTextContent('1.5 GB')
    expect(row('Size')).toHaveClass('font-mono')
    cleanup()
    const visuals = { ...chapter, pack: { ...chapter.pack, type: 'client-visuals' as const } }
    panels({ chapter: visuals })
    expect(row('Loader')).toHaveTextContent('Client visuals')
    expect(row('Loader')).not.toHaveClass('font-mono')
  })

  it('gives the wiki post one title line and four reserved text lines, with the link pinned below', () => {
    const onOpenWiki = vi.fn()
    panels({ onOpenWiki })
    const title = screen.getByText(chapter.wiki.title)
    const line = screen.getByText(chapter.wiki.line)
    expect(title).toHaveClass('line-clamp-1')
    expect(line).toHaveClass('line-clamp-4', 'min-h-[4lh]')
    const link = screen.getByRole('button', { name: 'Read the history →' })
    expect(link).toHaveClass('mt-auto')
    fireEvent.click(link)
    expect(onOpenWiki).toHaveBeenCalledOnce()
  })

  it('shows only the latest changelog entries, each clamped', () => {
    const changelog = ['1.0.5', '1.0.4', '1.0.3', '1.0.2', '1.0.1'].map((version) => ({
      version,
      summary: `Summary of ${version}`,
    }))
    panels({ chapter: { ...chapter, changelog } })
    expect(screen.getByText('1.0.5')).toBeInTheDocument()
    expect(screen.getByText('1.0.3')).toBeInTheDocument()
    expect(screen.queryByText('1.0.2')).toBeNull()
    expect(screen.getByText('Summary of 1.0.5')).toHaveClass('line-clamp-2')
  })

  it('shows the entries of the pack source by their first line, the rest in the tooltip', () => {
    const entries = ['1.0.5', '1.0.4', '1.0.3', '1.0.2'].map((version) => ({
      version,
      date: '2026-10-03',
      summary: `First of ${version}.`,
      details: `First of ${version}.\nSecond of ${version}.`,
    }))
    const manifested = { ...chapter, changelog: [{ version: '0.1', summary: 'Manifest entry.' }] }
    panels({ chapter: manifested, changelog: { chapterId: chapter.id, checked: true, entries } })
    expect(screen.getByText('1.0.5')).toHaveClass('text-accent', 'font-mono')
    expect(screen.getByText('First of 1.0.5.')).toHaveClass('line-clamp-2')
    expect(screen.getByText('First of 1.0.5.')).toHaveAttribute(
      'title',
      'First of 1.0.5.\nSecond of 1.0.5.',
    )
    expect(screen.queryByText('Second of 1.0.5.')).toBeNull()
    expect(screen.queryByText('1.0.2')).toBeNull()
    expect(screen.queryByText('Manifest entry.')).toBeNull()
  })

  it('falls back to the manifest changelog when the source has none or cannot be read', () => {
    const manifested = { ...chapter, changelog: [{ version: '0.1', summary: 'Manifest entry.' }] }
    const none = { chapterId: chapter.id, checked: true, entries: [] }
    panels({ chapter: manifested, changelog: none })
    expect(screen.getByText('Manifest entry.')).toBeInTheDocument()
    cleanup()
    panels({ chapter: manifested, changelog: { ...none, checked: false } })
    expect(screen.getByText('Manifest entry.')).toBeInTheDocument()
  })

  it('has one sentence for an empty changelog', () => {
    panels()
    expect(screen.getByText('No entries yet.')).toHaveClass('text-fg-faint', 'text-sm')
  })
})
