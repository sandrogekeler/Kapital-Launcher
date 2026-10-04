import type { Chapter } from '../../types'
import type { ChapterPageSpec } from '../../lib/usePages'
import { ChapterSettingsPanel } from '../settings/ChapterSettingsPanel'
import { LogsPanel } from '../logs/LogsPanel'
import { RunReportPanel } from './RunReportPanel'

interface Props {
  page: ChapterPageSpec
  chapter: Chapter
  onClose: () => void
}

/**
 * The page a chapter's slot names: its settings, its logs (issue 155) or the
 * report of its latest run. One module so the launcher loads the three
 * together when the first is asked for, and not before (App lazy-loads this,
 * which keeps them out of the first paint and the bundle budget,
 * scripts/check-bundle-size.mjs).
 */
export function ChapterPage({ page, chapter, onClose }: Props) {
  if (page.kind === 'settings') return <ChapterSettingsPanel chapter={chapter} onClose={onClose} />
  if (page.kind === 'logs') return <LogsPanel chapter={chapter} onClose={onClose} />
  return <RunReportPanel chapter={chapter} onClose={onClose} />
}
