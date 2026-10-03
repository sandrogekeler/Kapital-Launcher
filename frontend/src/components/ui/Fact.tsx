import { isPlaceholder } from '../../lib/manifest'

interface Props {
  label: string
  value: string
  /** What an unsettled value says, faint: "Unknown", or "Not installed". */
  unset?: string
  /** The value is a word, not data, so it is set in the UI face. */
  plain?: boolean
}

/**
 * One row of a fact list: a label in the UI face, a value in the data face
 * when it is a number or a version. An unsettled value is shown faint, in
 * words, rather than hidden, because an unsettled fact is information too.
 */
export function Fact({ label, value, unset = 'Unknown', plain }: Props) {
  const pending = isPlaceholder(value)
  return (
    <div className="flex justify-between text-sm">
      <span className="text-fg-muted">{label}</span>
      {pending ? (
        <span className="text-fg-faint">{unset}</span>
      ) : (
        <span className={plain ? '' : 'font-mono'}>{value}</span>
      )}
    </div>
  )
}
