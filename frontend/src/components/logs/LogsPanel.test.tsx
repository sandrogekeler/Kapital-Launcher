import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import * as Bindings from '../../../wailsjs/go/main/App'
import * as Runtime from '../../../wailsjs/runtime/runtime'
import { models } from '../../../wailsjs/go/models'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { EVENT_LOG_LIVE, useLogStore } from '../../stores/useLogStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { formatWhen } from '../../lib/runLogs'
import type { GamePhase, LiveLogEvent, RunLog, RunLogText } from '../../types'
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

/** The `log:live` listener the store registered, so a test can be Go. */
let handlers: Record<string, (e: unknown) => void> = {}
const live = (e: Partial<LiveLogEvent>) =>
  act(() => handlers[EVENT_LOG_LIVE]?.({ chapterId: 'frangfurd', lines: '', reset: false, ...e }))

const game = (phase: GamePhase) =>
  act(() =>
    useGameStore.getState().receive({
      chapterId: 'frangfurd',
      phase,
      since: '2026-10-04T10:00:00Z',
      startedAt: '2026-10-04T10:00:00Z',
    }),
  )

const dropdown = () => screen.getByRole('button', { name: /Choose a log/ })
const option = (name: RegExp | string) => screen.getByRole('option', { name })
const pick = (name: RegExp | string) => {
  fireEvent.click(dropdown())
  fireEvent.click(option(name))
}
const block = (name: string) => screen.findByLabelText(`${name}, masked`)

describe('LogsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    handlers = {}
    Object.assign(window, { go: {} })
    useEngineStore.setState({ instances: report(true) })
    useGameStore.setState({ states: {} })
    useLogStore.getState().clear()
    vi.mocked(Bindings.GetRunLogs).mockResolvedValue([latest, crash, dated] as never)
    vi.mocked(Bindings.ReadRunLog).mockResolvedValue(text() as never)
    vi.mocked(Bindings.WatchLiveLog).mockResolvedValue(
      text({ text: 'first\nsecond\n', lines: 2 }) as never,
    )
    vi.mocked(Bindings.StopLiveLog).mockResolvedValue()
    vi.mocked(Runtime.EventsOn).mockImplementation(((name: string, cb: (e: unknown) => void) => {
      handlers[name] = cb
      return () => undefined
    }) as never)
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  describe('the dropdown', () => {
    it('lists the live log, the most recent run, the dated runs, then the crash reports, with no kind word', async () => {
      panel()
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd logs')
      await block('latest.log')
      expect(Bindings.GetRunLogs).toHaveBeenCalledExactlyOnceWith('frangfurd')
      fireEvent.click(dropdown())
      const entries = within(screen.getByRole('listbox')).getAllByRole('option')
      expect(entries.map((e) => e.textContent)).toEqual([
        'Live log',
        `Most recent ${formatWhen(latest.modifiedAt)} Crashed`,
        formatWhen(dated.modifiedAt),
        `Crash report ${formatWhen(crash.modifiedAt)}`,
      ])
      // Everything is a log: no entry is called one, and no size is drawn.
      for (const e of entries.slice(1)) expect(e).not.toHaveTextContent(/\blog\b/i)
      expect(screen.queryByText(/KB|MB/)).toBeNull()
      expect(within(entries[1]!).getByText('Crashed')).toHaveClass('text-danger')
      expect(within(entries[3]!).queryByText('Crashed')).toBeNull()
    })

    it('opens on the most recent run when the game is not running, and reads only that', async () => {
      panel()
      expect(await block('latest.log')).toHaveTextContent('Setting user: [player]')
      expect(Bindings.ReadRunLog).toHaveBeenCalledExactlyOnceWith(
        'frangfurd',
        'log',
        'latest.log',
        0,
      )
      expect(Bindings.WatchLiveLog).not.toHaveBeenCalled()
      expect(Runtime.EventsOn).not.toHaveBeenCalled()
      expect(dropdown()).toHaveAccessibleName(/Most recent/)
      // A past run has no live status line.
      expect(screen.queryByText('Live')).toBeNull()
      expect(screen.queryByText(/The game is not running/)).toBeNull()
    })

    it('opens on the live log while the game is starting or running', async () => {
      for (const phase of ['starting', 'running'] as const) {
        cleanup()
        vi.clearAllMocks()
        useLogStore.getState().clear()
        useGameStore.setState({ states: {} })
        game(phase)
        panel()
        expect(await block('The live log')).toHaveTextContent('first second')
        expect(Bindings.WatchLiveLog).toHaveBeenCalledExactlyOnceWith('frangfurd')
        expect(Runtime.EventsOn).toHaveBeenCalledWith(EVENT_LOG_LIVE, expect.any(Function))
        expect(Bindings.ReadRunLog).not.toHaveBeenCalled()
        expect(dropdown()).toHaveAccessibleName(/Live log/)
      }
    })

    it('opens on the newest log when there is no latest.log, and on the live log with no logs while the game starts', async () => {
      vi.mocked(Bindings.GetRunLogs).mockResolvedValue([crash, dated] as never)
      panel()
      await waitFor(() =>
        expect(Bindings.ReadRunLog).toHaveBeenCalledWith('frangfurd', 'log', dated.name, 0),
      )
      cleanup()
      useLogStore.getState().clear()
      vi.mocked(Bindings.GetRunLogs).mockResolvedValue([] as never)
      vi.mocked(Bindings.WatchLiveLog).mockResolvedValue(text({ text: '' }) as never)
      game('starting')
      panel()
      expect(await screen.findByText('Nothing has been written yet.')).toBeVisible()
      fireEvent.click(dropdown())
      expect(screen.getAllByRole('option')).toHaveLength(1)
    })

    it('opens a dated run and a crash report by their kind and name', async () => {
      panel()
      await block('latest.log')
      vi.mocked(Bindings.ReadRunLog).mockResolvedValue(
        text({ kind: 'crash', name: crash.name, text: 'Description: boom\n' }) as never,
      )
      pick(/Crash report/)
      expect(await block(crash.name)).toHaveTextContent('boom')
      expect(Bindings.ReadRunLog).toHaveBeenLastCalledWith('frangfurd', 'crash', crash.name, 0)
      vi.mocked(Bindings.ReadRunLog).mockResolvedValue(
        text({ name: dated.name, text: 'old run\n' }) as never,
      )
      pick(new RegExp(formatWhen(dated.modifiedAt)))
      expect(await block(dated.name)).toHaveTextContent('old run')
      expect(Bindings.ReadRunLog).toHaveBeenLastCalledWith('frangfurd', 'log', dated.name, 0)
    })
  })

  describe('a past run', () => {
    it('shows the end of the file, masked, with the note and Copy and nothing earlier to load', async () => {
      panel()
      expect(await block('latest.log')).toHaveTextContent('Setting user: [player]')
      expect(screen.getByText(/Your name, folders and server addresses are masked/)).toBeVisible()
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeEnabled()
      expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
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
      await block('latest.log')
      fireEvent.click(screen.getByRole('button', { name: 'Load earlier' }))
      await waitFor(() =>
        expect(screen.getByLabelText('latest.log, masked')).toHaveTextContent(
          'line 1 line 2 line 3 line 4',
        ),
      )
      // The offset of the chunk shown is what the next read asks to end at.
      expect(Bindings.ReadRunLog).toHaveBeenLastCalledWith('frangfurd', 'log', 'latest.log', 14)
      expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
    })

    it('copies the text it shows, and says when it could not', async () => {
      vi.mocked(Runtime.ClipboardSetText).mockResolvedValueOnce(true).mockResolvedValueOnce(false)
      panel()
      await block('latest.log')
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      await screen.findByRole('button', { name: 'Copied' })
      expect(Runtime.ClipboardSetText).toHaveBeenCalledWith('[main/INFO]: Setting user: [player]\n')
      await waitFor(() => expect(screen.getByRole('button', { name: 'Copy log' })).toBeVisible(), {
        timeout: 3000,
      })
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeVisible()
    })

    it("shows Go's refusal of a file in the viewer and keeps the dropdown", async () => {
      panel()
      await block('latest.log')
      vi.mocked(Bindings.ReadRunLog).mockRejectedValue('Frangfurd: the log is too large to unpack')
      pick(new RegExp(formatWhen(dated.modifiedAt)))
      expect(await screen.findByRole('alert')).toHaveTextContent('too large to unpack')
      expect(dropdown()).toBeVisible()
      expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
    })
  })

  describe('the live log', () => {
    const openLive = async () => {
      game('running')
      panel()
      return block('The live log')
    }

    it('appends the lines Go sends, and replaces them on a reset', async () => {
      const view = await openLive()
      live({ lines: 'third\nfourth\n' })
      expect(view).toHaveTextContent('first second third fourth')
      live({ lines: 'new run\n', reset: true })
      expect(view).toHaveTextContent('new run')
      expect(view).not.toHaveTextContent('first')
      live({ lines: 'and more\n' })
      expect(view).toHaveTextContent('new run and more')
    })

    it('drops an event of another chapter or of none, and one that arrives after it was left', async () => {
      const view = await openLive()
      live({ chapterId: 'luxemburg', lines: 'elsewhere\n' })
      live({ chapterId: '', lines: 'nowhere\n' })
      expect(view).not.toHaveTextContent('elsewhere')
      expect(view).not.toHaveTextContent('nowhere')
    })

    it('keeps the lines Go sent while the first text was on its way, after it', async () => {
      let resolve!: (t: RunLogText) => void
      vi.mocked(Bindings.WatchLiveLog).mockReturnValue(
        new Promise<RunLogText>((r) => (resolve = r)) as never,
      )
      game('running')
      panel()
      await waitFor(() => expect(handlers[EVENT_LOG_LIVE]).toBeDefined())
      live({ lines: 'arrived early\n' })
      await act(async () => resolve(text({ text: 'first\n' })))
      expect(await block('The live log')).toHaveTextContent('first arrived early')
    })

    it('holds only the newest 5000 lines', async () => {
      const view = await openLive()
      const many = Array.from({ length: 5100 }, (_, i) => `line ${i}`).join('\n') + '\n'
      live({ lines: many })
      expect(view).not.toHaveTextContent(/line 0\b/)
      expect(view).not.toHaveTextContent(/\bfirst\b/)
      expect(view.textContent?.split('\n')).toHaveLength(5001)
      expect(view).toHaveTextContent('line 5099')
    })

    it('says Live with a pulsing dot while the game runs, and still when it is only starting', async () => {
      await openLive()
      const status = screen.getByRole('status')
      expect(status).toHaveTextContent('Live')
      expect(status).toHaveClass('text-accent')
      const dot = status.querySelector('[aria-hidden="true"]')!
      // The pulse is off for a player who asked for less motion.
      expect(dot).toHaveClass('animate-pulse', 'motion-reduce:animate-none')
      game('starting')
      expect(screen.getByRole('status')).toHaveTextContent('Live')
      expect(screen.queryByText(/The game is not running/)).toBeNull()
    })

    it('says it is the end of the last run when the game is not running, and Live again once it is', async () => {
      panel()
      await block('latest.log')
      pick('Live log')
      await block('The live log')
      expect(
        screen.getByText('The game is not running. This is the end of its last run.'),
      ).toBeVisible()
      expect(screen.queryByText('Live')).toBeNull()
      game('running')
      expect(screen.getByRole('status')).toHaveTextContent('Live')
      game('closed')
      expect(screen.getByText(/The game is not running/)).toBeVisible()
    })

    it('does not start again when the live log is chosen while it is followed', async () => {
      await openLive()
      pick('Live log')
      expect(Bindings.WatchLiveLog).toHaveBeenCalledTimes(1)
    })

    it('stops following when another entry is chosen, and listens no more', async () => {
      const view = await openLive()
      expect(Bindings.StopLiveLog).not.toHaveBeenCalled()
      pick(/Most recent/)
      expect(Bindings.StopLiveLog).toHaveBeenCalledExactlyOnceWith('frangfurd')
      expect(Runtime.EventsOff).toHaveBeenCalledWith(EVENT_LOG_LIVE)
      expect(await block('latest.log')).toHaveTextContent('Setting user')
      live({ lines: 'late\n' })
      expect(view).not.toBeInTheDocument()
      expect(screen.getByLabelText('latest.log, masked')).not.toHaveTextContent('late')
    })

    it('stops following when the page closes, and drops its lines', async () => {
      const { unmount } = await (async () => {
        game('running')
        const rendered = panel()
        await block('The live log')
        return rendered
      })()
      unmount()
      expect(Bindings.StopLiveLog).toHaveBeenCalledExactlyOnceWith('frangfurd')
      expect(Runtime.EventsOff).toHaveBeenCalledWith(EVENT_LOG_LIVE)
      expect(useLogStore.getState().live).toBeNull()
      expect(useLogStore.getState().liveLines).toEqual([])
    })

    it('stops following the chapter it was on when the page is opened for another', async () => {
      await openLive()
      cleanup()
      expect(Bindings.StopLiveLog).toHaveBeenCalledExactlyOnceWith('frangfurd')
    })

    it('copies what is shown', async () => {
      vi.mocked(Runtime.ClipboardSetText).mockResolvedValue(true)
      await openLive()
      live({ lines: 'third\n' })
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      await screen.findByRole('button', { name: 'Copied' })
      expect(Runtime.ClipboardSetText).toHaveBeenCalledWith('first\nsecond\nthird\n')
    })

    it('has no Load earlier', async () => {
      vi.mocked(Bindings.WatchLiveLog).mockResolvedValue(
        text({ truncated: true, offset: 9 }) as never,
      )
      await openLive()
      expect(screen.queryByRole('button', { name: 'Load earlier' })).toBeNull()
    })

    it("shows Go's refusal to follow, and listens to nothing", async () => {
      vi.mocked(Bindings.WatchLiveLog).mockRejectedValue('Frangfurd: the log could not be read')
      game('running')
      panel()
      expect(await screen.findByRole('alert')).toHaveTextContent('could not be read')
      expect(Runtime.EventsOff).toHaveBeenCalledWith(EVENT_LOG_LIVE)
    })

    describe('following the end', () => {
      // jsdom lays nothing out: the block's geometry is whatever a test says.
      const geometry = (el: HTMLElement, h: { scrollHeight: number; clientHeight: number }) => {
        Object.defineProperty(el, 'scrollHeight', { configurable: true, value: h.scrollHeight })
        Object.defineProperty(el, 'clientHeight', { configurable: true, value: h.clientHeight })
      }
      const grow = (el: HTMLElement, scrollHeight: number) =>
        Object.defineProperty(el, 'scrollHeight', { configurable: true, value: scrollHeight })

      it('goes to the end as lines arrive while the player is at it', async () => {
        const view = await openLive()
        geometry(view, { scrollHeight: 1000, clientHeight: 100 })
        view.scrollTop = 900
        fireEvent.scroll(view)
        grow(view, 1200)
        live({ lines: 'more\n' })
        expect(view.scrollTop).toBe(1200)
        expect(screen.queryByRole('button', { name: 'Jump to latest' })).toBeNull()
      })

      it('stays where it is when the player scrolled up, and offers Jump to latest', async () => {
        const view = await openLive()
        geometry(view, { scrollHeight: 1000, clientHeight: 100 })
        view.scrollTop = 300
        fireEvent.scroll(view)
        const jump = await screen.findByRole('button', { name: 'Jump to latest' })
        grow(view, 1200)
        live({ lines: 'more\n' })
        expect(view.scrollTop).toBe(300)

        fireEvent.click(jump)
        expect(view.scrollTop).toBe(1200)
        expect(screen.queryByRole('button', { name: 'Jump to latest' })).toBeNull()
        // Followed again.
        grow(view, 1400)
        live({ lines: 'and more\n' })
        expect(view.scrollTop).toBe(1400)
      })

      it('offers no Jump to latest on a past run', async () => {
        panel()
        const view = await block('latest.log')
        geometry(view, { scrollHeight: 1000, clientHeight: 100 })
        view.scrollTop = 300
        fireEvent.scroll(view)
        expect(screen.queryByRole('button', { name: 'Jump to latest' })).toBeNull()
      })
    })
  })

  describe('the page', () => {
    it('says there are no logs yet when the instance has none and the game is not running', async () => {
      vi.mocked(Bindings.GetRunLogs).mockResolvedValue([] as never)
      panel()
      expect(
        await screen.findByText('No logs yet. They appear after the first Play.'),
      ).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Choose a log/ })).toBeNull()
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
      expect(Bindings.WatchLiveLog).not.toHaveBeenCalled()
    })

    it('opens the instance folder from the header, and shows why it would not open', async () => {
      vi.mocked(Bindings.OpenInstanceFolder)
        .mockResolvedValueOnce()
        .mockRejectedValueOnce('no file manager')
      panel()
      await block('latest.log')
      fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
      await waitFor(() => expect(Bindings.OpenInstanceFolder).toHaveBeenCalledWith('frangfurd'))
      fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
      expect(await screen.findByRole('alert')).toHaveTextContent('no file manager')
    })

    it('closes with Back and with Escape, and drops what it read', async () => {
      const onClose = vi.fn()
      const { unmount } = panel(onClose)
      await block('latest.log')
      fireEvent.click(screen.getByRole('button', { name: 'Back' }))
      fireEvent.keyDown(window, { key: 'Escape' })
      expect(onClose).toHaveBeenCalledTimes(2)
      unmount()
      expect(useLogStore.getState().logs).toBeNull()
      expect(useLogStore.getState().opened).toBeNull()
    })

    it('keeps Escape for the dropdown while it is open', async () => {
      const onClose = vi.fn()
      panel(onClose)
      await block('latest.log')
      fireEvent.click(dropdown())
      fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Escape' })
      expect(onClose).not.toHaveBeenCalled()
      expect(screen.queryByRole('listbox')).toBeNull()
    })
  })
})
