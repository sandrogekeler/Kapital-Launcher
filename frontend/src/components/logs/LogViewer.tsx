import { useLayoutEffect, useRef, useState } from 'react'
import type { Chapter } from '../../types'
import { useLogStore } from '../../stores/useLogStore'
import { errMsg } from '../../lib/ipc'
import { formatBytes, formatWhen, kindLabel } from '../../lib/runLogs'
import { Button } from '../ui/Button'
import { CopyLogButton } from '../ui/CopyLogButton'
import type { CopyResult } from '../ui/CopyLogButton'
import { Hint, ErrorLine } from '../ui/Notes'
import { LogBlock } from '../run/RunReportParts'

/**
 * The file chosen in the list, as Go sent it: whole lines, already masked, from
 * the end of the file, with Load earlier for the stretch before. The block
 * opens on the last line, where a crash is; text loaded in front keeps what was
 * being read where it was. Copy puts on the clipboard what is shown, no more.
 */
export function LogViewer({ chapter }: { chapter: Chapter }) {
  const selected = useLogStore((s) => s.selected)
  const opened = useLogStore((s) => s.opened)
  const reading = useLogStore((s) => s.reading)
  const readError = useLogStore((s) => s.readError)
  const loadEarlier = useLogStore((s) => s.loadEarlier)
  const copy = useLogStore((s) => s.copy)
  const [copied, setCopied] = useState<CopyResult | null>(null)

  const end = useRef<HTMLSpanElement>(null)
  // What the block held when it last changed, to tell text put in front of it
  // from a new file.
  const last = useRef<{ key: string; offset: number; height: number } | null>(null)
  // The scrolling element is the sentinel's parent: Scrollable does not hand
  // its ref out. A file opens on its last line, where a crash is; text put in
  // front of it leaves the line that was first at the top, so reading goes on
  // upward from there.
  useLayoutEffect(() => {
    const block = end.current?.parentElement
    if (!block || !opened) {
      last.current = null
      return
    }
    const key = `${opened.kind}:${opened.name}`
    const prev = last.current
    const prepended = prev !== null && prev.key === key && opened.offset < prev.offset
    block.scrollTop = prepended ? block.scrollHeight - prev.height : block.scrollHeight
    last.current = { key, offset: opened.offset, height: block.scrollHeight }
  }, [opened])

  if (!selected) return null

  const onCopy = async () => {
    try {
      await copy()
      setCopied({})
    } catch (e) {
      setCopied({ error: errMsg(e) })
    }
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <span className="text-fg font-mono text-sm select-text">{selected.name}</span>
        <span className="text-fg-muted text-xs">
          {kindLabel(selected)}, {formatWhen(selected.modifiedAt)}, {formatBytes(selected.size)}
        </span>
      </div>
      {opened ? (
        <>
          <LogBlock height="h-96" label={`${selected.name}, masked`}>
            {opened.text || 'This file is empty.'}
            <span ref={end} />
          </LogBlock>
          <div className="flex flex-wrap items-center gap-3">
            {opened.truncated && (
              <Button onClick={() => void loadEarlier(chapter.id)} disabled={reading}>
                <span>Load earlier</span>
              </Button>
            )}
            <CopyLogButton onClick={() => void onCopy()} result={copied} disabled={reading} />
          </div>
          <Hint>
            Your name, folders and server addresses are masked, here and in what you copy.
          </Hint>
        </>
      ) : (
        !readError && <p className="text-fg-muted m-0 text-sm">Reading the file.</p>
      )}
      {readError && <ErrorLine>{readError}</ErrorLine>}
    </div>
  )
}
