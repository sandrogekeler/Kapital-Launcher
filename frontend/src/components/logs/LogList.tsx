import type { RunLog } from '../../types'
import { formatBytes, formatWhen, kindLabel, logKey } from '../../lib/runLogs'
import { Hint } from '../ui/Notes'
import { Scrollable } from '../ui/Scrollable'

interface Props {
  logs: readonly RunLog[]
  /** The key (kind and name) of the file open in the viewer. */
  openKey: string | null
  onOpen: (log: RunLog) => void
}

/**
 * A chapter's game logs and crash reports, newest first, one button per file:
 * when it was written in the player's locale, what it is, how big, and a mark
 * on a log whose run ended in a crash. The list scrolls inside a box of its own
 * so a long history does not push the viewer out of the page.
 */
export function LogList({ logs, openKey, onOpen }: Props) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="bg-sunken border-line box-content flex max-h-64 min-h-0 overflow-hidden rounded-md border">
        <Scrollable as="ul" aria-label="Logs and crash reports" className="m-0 list-none p-1">
          {logs.map((log) => {
            const key = logKey(log)
            return (
              <li key={key}>
                <button
                  type="button"
                  onClick={() => onOpen(log)}
                  aria-pressed={key === openKey}
                  className={`duration-fast ease-standard flex w-full cursor-pointer items-baseline gap-3 rounded-sm px-3 py-2 text-left transition-colors ${
                    key === openKey ? 'bg-accent-wash' : 'hover:bg-hover'
                  }`}
                >
                  <span className="text-fg w-44 shrink-0 text-sm">
                    {formatWhen(log.modifiedAt)}
                  </span>
                  <span className="text-fg-muted min-w-0 grow truncate text-xs">
                    {kindLabel(log)}
                  </span>
                  {log.crashed && <span className="text-danger text-xs font-medium">Crashed</span>}
                  <span className="text-fg-faint w-16 shrink-0 text-right font-mono text-xs">
                    {formatBytes(log.size)}
                  </span>
                </button>
              </li>
            )
          })}
        </Scrollable>
      </div>
      <Hint>Older logs are dated when the game archived them, at the start of the next run.</Hint>
    </div>
  )
}
