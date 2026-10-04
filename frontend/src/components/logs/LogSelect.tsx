import { useMemo } from 'react'
import type { RunLog } from '../../types'
import { selectOpenKey, useLogStore } from '../../stores/useLogStore'
import { LIVE_KEY, logKey, logOptions } from '../../lib/runLogs'
import { Select } from '../ui/Select'

interface Props {
  chapterId: string
  logs: readonly RunLog[]
}

/**
 * The logs dropdown (issue 155): the live log first, then the most recent run,
 * the dated runs and the crash reports, each by when it was written in the
 * player's locale, with a red mark on a run that crashed. Choosing one reads it,
 * or, for the live log, starts following it.
 */
export function LogSelect({ chapterId, logs }: Props) {
  const choice = useLogStore(selectOpenKey)
  const open = useLogStore((s) => s.open)
  const watch = useLogStore((s) => s.watch)
  const options = useMemo(() => logOptions(logs), [logs])

  const onChange = (key: string) => {
    if (key === LIVE_KEY) {
      // Already following it: nothing to start again.
      if (choice !== LIVE_KEY) void watch(chapterId)
      return
    }
    const log = logs.find((l) => logKey(l) === key)
    if (log) void open(chapterId, log)
  }

  return <Select label="Choose a log" value={choice} options={options} onChange={onChange} />
}
