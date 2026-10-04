import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import * as Bindings from '../../../wailsjs/go/main/App'
import * as Runtime from '../../../wailsjs/runtime/runtime'
import { models } from '../../../wailsjs/go/models'
import { useEngineStore } from '../../stores/useEngineStore'
import { useLogStore } from '../../stores/useLogStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { formatWhen } from '../../lib/runLogs'
import type { RunLog, RunLogText } from '../../types'
import { LogsPanel } from './LogsPanel'

vi.mock('../../../wailsjs/go/main/App')
vi.mock('../../../wailsjs/runtime/runtime')

const chapter = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!

const latest: RunLog = {
  kind: 'log',
  name: 'latest.log',
  modifiedAt: '2026-10-03T15:00:00Z',
  size: 4096,
  crashed: true,
}
const dated: RunLog = {
  kind: 'log',
  name: '2026-10-02-1.log.gz',
  modifiedAt: '2026-10-02T20:30:00Z',
  size: 2_500_000,
  crashed: false,
}
const crash: RunLog = {
  kind: 'crash',
  name: 'crash-2026-10-03_16.59.00-client.txt',
  modifiedAt: '2026-10-03T14:59:00Z',
  size: 100,
  crashed: false,
}

const text = (over: Partial<RunLogText> = {}): RunLogText => ({
  kind: 'log',
  name: 'latest.log',
  text: '[main/INFO]: Setting user: [player]\n',
  offset: 0,
  size: 40,
  lines: 1,
  truncated: false,
  ...over,
})

const report = (present: boolean) =>
  models.InstanceReport.createFrom({
    root: 'C:/Prism',
    dir: 'C:/Prism/instances',
    present: { frangfurd: present },
    sizeBytes: {},
  })

const panel = (onClose = () => undefined) =>
  render(<LogsPanel chapter={chapter} onClose={onClose} />)

describe('LogsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useEngineStore.setState({ instances: report(true) })
    useLogStore.getState().clear()
    vi.mocked(Bindings.GetRunLogs).mockResolvedValue([latest, crash, dated] as never)
    vi.mocked(Bindings.ReadRunLog).mockResolvedValue(text() as never)
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('lists the files newest first with when, what, how big and a Crashed mark', async () => {
    panel()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd logs')
    const list = await screen.findByRole('list', { name: 'Logs and crash reports' })
    expect(Bindings.GetRunLogs).toHaveBeenCalledExactlyOnceWith('frangfurd')
    const rows = within(list).getAllByRole('button')
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent(formatWhen(latest.modifiedAt))
    expect(rows[0]).toHaveTextContent('Log, current or most recent run')
    expect(rows[0]).toHaveTextContent('Crashed')
    expect(rows[0]).toHaveTextContent('4.0 KB')
    expect(rows[1]).toHaveTextContent('Crash report')
    expect(rows[1]).not.toHaveTextContent('Crashed')
    expect(rows[2]).toHaveTextContent('2.4 MB')
    // Nothing is opened until a row is chosen, and no file was read.
    expect(Bindings.ReadRunLog).not.toHaveBeenCalled()
    expect(screen.queryByLabelText(/, masked/)).toBeNull()
  })

  it('opens a file at its end in the viewer, masked, with the note and Copy', async () => {
    panel()
    fireEvent.click(await screen.findByRole('button', { name: /Log, current or most recent run/ }))
    const block = await screen.findByLabelText('latest.log, masked')
    expect(block).toHaveTextContent('Setting user: [player]')
    expect(Bindings.ReadRunLog).toHaveBeenCalledExactlyOnceWith('frangfurd', 'log', 'latest.log', 0)
    expect(screen.getByRole('button', { name: /Log, current or most recent run/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(screen.getByText(/Your name, folders and server addresses are masked/)).toBeVisible()
    // The whole file is shown: nothing earlier to load.
    expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
  })

  it('opens a crash report by its kind and name', async () => {
    vi.mocked(Bindings.ReadRunLog).mockResolvedValue(
      text({ kind: 'crash', name: crash.name, text: 'Description: boom\n' }) as never,
    )
    panel()
    fireEvent.click(await screen.findByRole('button', { name: /Crash report/ }))
    expect(await screen.findByLabelText(`${crash.name}, masked`)).toHaveTextContent('boom')
    expect(Bindings.ReadRunLog).toHaveBeenCalledWith('frangfurd', 'crash', crash.name, 0)
  })

  it('loads the stretch before what is shown and puts it in front', async () => {
    vi.mocked(Bindings.ReadRunLog)
      .mockResolvedValueOnce(
        text({
          text: 'line 3\nline 4\n',
          offset: 14,
          lines: 2,
          truncated: true,
          size: 28,
        }) as never,
      )
      .mockResolvedValueOnce(
        text({
          text: 'line 1\nline 2\n',
          offset: 0,
          lines: 2,
          truncated: false,
          size: 28,
        }) as never,
      )
    panel()
    fireEvent.click(await screen.findByRole('button', { name: /Log, current or most recent run/ }))
    await screen.findByLabelText('latest.log, masked')
    fireEvent.click(screen.getByRole('button', { name: 'Load earlier' }))
    await waitFor(() =>
      expect(screen.getByLabelText('latest.log, masked')).toHaveTextContent(
        'line 1 line 2 line 3 line 4',
      ),
    )
    // The offset of the chunk shown is what the next read asks to end at.
    expect(Bindings.ReadRunLog).toHaveBeenLastCalledWith('frangfurd', 'log', 'latest.log', 14)
    // Back at the start of the file: nothing more to load.
    expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
  })

  it('copies the text it shows, and says when it could not', async () => {
    vi.mocked(Runtime.ClipboardSetText).mockResolvedValueOnce(true).mockResolvedValueOnce(false)
    panel()
    fireEvent.click(await screen.findByRole('button', { name: /Log, current or most recent run/ }))
    await screen.findByLabelText('latest.log, masked')
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    await screen.findByRole('button', { name: 'Copied' })
    expect(Runtime.ClipboardSetText).toHaveBeenCalledWith('[main/INFO]: Setting user: [player]\n')
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy log' })).toBeVisible(), {
      timeout: 3000,
    })
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeVisible()
  })

  it("shows Go's refusal of a file in the viewer and keeps the list", async () => {
    vi.mocked(Bindings.ReadRunLog).mockRejectedValue('Frangfurd: the log is too large to unpack')
    panel()
    fireEvent.click(await screen.findByRole('button', { name: /^.*2\.4 MB$/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('too large to unpack')
    expect(screen.getByRole('list', { name: 'Logs and crash reports' })).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
  })

  it('says there are no logs yet when the instance has none', async () => {
    vi.mocked(Bindings.GetRunLogs).mockResolvedValue([] as never)
    panel()
    expect(
      await screen.findByText('No logs yet. They appear after the first Play.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('list')).toBeNull()
  })

  it('says a chapter that is not installed has no logs, and reads nothing', async () => {
    useEngineStore.setState({ instances: report(false) })
    panel()
    expect(screen.getByText(/Frangfurd is not installed yet, so it has no logs/)).toBeVisible()
    expect(screen.getByRole('button', { name: 'Open folder' })).toBeDisabled()
    expect(Bindings.GetRunLogs).not.toHaveBeenCalled()
  })

  it('says so when the instance could not be looked for', () => {
    useEngineStore.setState({ instances: null })
    panel()
    expect(screen.getByText(/The instance could not be looked for/)).toBeVisible()
    expect(Bindings.GetRunLogs).not.toHaveBeenCalled()
  })

  it('shows what Go refused the list with in the error style', async () => {
    vi.mocked(Bindings.GetRunLogs).mockRejectedValue('Frangfurd: open logs: denied')
    panel()
    expect(await screen.findByRole('alert')).toHaveTextContent('open logs: denied')
  })

  it('degrades without a bridge', async () => {
    Reflect.deleteProperty(window, 'go')
    panel()
    expect(await screen.findByText('The logs can only be read in the app window.')).toBeVisible()
    expect(Bindings.GetRunLogs).not.toHaveBeenCalled()
  })

  it('opens the instance folder from the header, and shows why it would not open', async () => {
    vi.mocked(Bindings.OpenInstanceFolder)
      .mockResolvedValueOnce()
      .mockRejectedValueOnce('no file manager')
    panel()
    await screen.findByRole('list', { name: 'Logs and crash reports' })
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    await waitFor(() => expect(Bindings.OpenInstanceFolder).toHaveBeenCalledWith('frangfurd'))
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('no file manager')
  })

  it('closes with Back and with Escape, and drops what it read', async () => {
    const onClose = vi.fn()
    const { unmount } = panel(onClose)
    fireEvent.click(await screen.findByRole('button', { name: /Log, current or most recent run/ }))
    await screen.findByLabelText('latest.log, masked')
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(2)
    unmount()
    expect(useLogStore.getState().logs).toBeNull()
    expect(useLogStore.getState().opened).toBeNull()
  })
})
