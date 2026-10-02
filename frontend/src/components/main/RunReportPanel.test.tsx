import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as Bindings from '../../../wailsjs/go/main/App'
import { RunReportPanel } from './RunReportPanel'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { useGameStore } from '../../stores/useGameStore'
import type { GameState, RunReport } from '../../types'

vi.mock('../../../wailsjs/go/main/App')
vi.mock('../../../wailsjs/runtime/runtime')

const chapter = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!

const crashed: GameState = {
  chapterId: chapter.id,
  phase: 'crashed',
  since: '2026-10-02T10:00:31Z',
  startedAt: '2026-10-02T10:00:00Z',
  exitCode: 1,
}

const runReport = (over: Partial<RunReport> = {}): RunReport => ({
  game: crashed,
  phases: [
    { phase: 'starting', ms: 0 },
    { phase: 'mods', ms: 4200 },
    { phase: 'window', ms: 9800 },
  ],
  logTail: '[Render thread/ERROR]: Reported exception thrown!\n',
  logLines: 1,
  logTruncated: false,
  crashReport: 'crash-2026-10-02_10.00.00-client.txt',
  consoleAvailable: false,
  ...over,
})

const attachBridge = () => Object.assign(window, { go: {} })
const detachBridge = () => {
  delete (window as unknown as { go?: unknown }).go
}

describe('RunReportPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    attachBridge()
    useGameStore.setState({ states: { [chapter.id]: crashed } })
    vi.mocked(Bindings.GetRunReport).mockResolvedValue(runReport() as never)
  })
  afterEach(() => {
    cleanup()
    detachBridge()
  })

  it('reads the report once when it opens and shows what the launcher knows', async () => {
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    expect(await screen.findByLabelText('Timeline')).toHaveTextContent(
      'mods 4.2 s, window 9.8 s, crashed 31 s',
    )
    expect(Bindings.GetRunReport).toHaveBeenCalledExactlyOnceWith(chapter.id)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd run report')
    expect(screen.getByText('○ The game stopped')).toHaveClass('text-danger')
    expect(screen.getByLabelText("The end of the game's log")).toHaveTextContent(
      'Reported exception thrown!',
    )
    expect(screen.getByText('crash-2026-10-02_10.00.00-client.txt')).toBeInTheDocument()
    expect(screen.getByText(/The last|All 1 line/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: "Show Prism's console" })).toBeNull()
  })

  it('closes with Back and with Escape', async () => {
    const onClose = vi.fn()
    render(<RunReportPanel chapter={chapter} onClose={onClose} />)
    await screen.findByLabelText('Timeline')
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('reads again on the chapter next game:state while it is open, and on nothing else', async () => {
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    await screen.findByLabelText('Timeline')

    // Another chapter's event is not this one's.
    act(() =>
      useGameStore.getState().receive({ ...crashed, chapterId: 'luxemburg', phase: 'mods' }),
    )
    expect(Bindings.GetRunReport).toHaveBeenCalledTimes(1)

    vi.mocked(Bindings.GetRunReport).mockResolvedValue(
      runReport({ game: { ...crashed, phase: 'starting' }, logTail: '', logLines: 0 }) as never,
    )
    act(() => useGameStore.getState().receive({ ...crashed, phase: 'starting' }))
    await waitFor(() => expect(Bindings.GetRunReport).toHaveBeenCalledTimes(2))
    expect(await screen.findByText(/wrote no log for this run/)).toBeInTheDocument()
  })

  it('shows the refusal of a real backend', async () => {
    vi.mocked(Bindings.GetRunReport).mockRejectedValue('Frangfurd: there is no run to report on')
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('there is no run to report on')
  })

  it('says it needs the app window when there is no bridge, and asks Go nothing', async () => {
    detachBridge()
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    expect(await screen.findByText(/only be read in the app window/)).toBeInTheDocument()
    expect(Bindings.GetRunReport).not.toHaveBeenCalled()
  })

  it('opens the folder, and copies the log with the button saying how it went', async () => {
    vi.mocked(Bindings.OpenInstanceFolder).mockResolvedValue()
    vi.mocked(Bindings.CopyRedactedLog).mockResolvedValue(12)
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    await screen.findByLabelText('Timeline')

    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    await waitFor(() => expect(Bindings.OpenInstanceFolder).toHaveBeenCalledWith(chapter.id))
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
    expect(screen.queryByText(/to the clipboard/)).toBeNull()

    vi.mocked(Bindings.CopyRedactedLog).mockRejectedValue('the log has nothing in it yet')
    fireEvent.click(screen.getByRole('button', { name: 'Copied' }))
    expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
    expect(screen.queryByText('the log has nothing in it yet')).toBeNull()
  })

  it('says why the folder would not open, beside the buttons', async () => {
    vi.mocked(Bindings.OpenInstanceFolder).mockRejectedValue('could not open the folder')
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    await screen.findByLabelText('Timeline')
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    expect(await screen.findByText('could not open the folder')).toHaveClass('text-danger')
  })

  it("offers Prism's console only when the report says it can be shown", async () => {
    vi.mocked(Bindings.GetRunReport).mockResolvedValue(
      runReport({ consoleAvailable: true }) as never,
    )
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    expect(await screen.findByRole('button', { name: "Show Prism's console" })).toBeInTheDocument()
  })

  it('has no console button when the report says there is none', async () => {
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    await screen.findByLabelText('Timeline')
    expect(screen.queryByRole('button', { name: "Show Prism's console" })).toBeNull()
  })

  it("shows Prism's console through the binding, and says when it is gone or refused", async () => {
    vi.mocked(Bindings.GetRunReport).mockResolvedValue(
      runReport({ consoleAvailable: true }) as never,
    )
    vi.mocked(Bindings.ShowPrismConsole).mockResolvedValue(true)
    render(<RunReportPanel chapter={chapter} onClose={() => undefined} />)
    fireEvent.click(await screen.findByRole('button', { name: "Show Prism's console" }))
    await waitFor(() => expect(Bindings.ShowPrismConsole).toHaveBeenCalledWith(chapter.id))
    expect(screen.queryByText(/no longer open/)).toBeNull()

    vi.mocked(Bindings.ShowPrismConsole).mockResolvedValue(false)
    fireEvent.click(screen.getByRole('button', { name: "Show Prism's console" }))
    expect(await screen.findByText("Prism's console is no longer open")).toHaveClass('text-danger')

    vi.mocked(Bindings.ShowPrismConsole).mockRejectedValue('no chapter "x"')
    fireEvent.click(screen.getByRole('button', { name: "Show Prism's console" }))
    expect(await screen.findByText('no chapter "x"')).toHaveClass('text-danger')
  })
})
