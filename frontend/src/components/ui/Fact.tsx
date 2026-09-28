import { isPlaceholder } from '../../lib/manifest'

interface Props {
  label: string
  value: string
}

/**
 * One row of a fact list: a label in the UI face, a value in the data face.
 * A placeholder value is shown faint rather than hidden, because an unsettled
 * fact is information too.
 */
export function Fact({ label, value }: Props) {
  const pending = isPlaceholder(value)
  return (
    <div className="flex justify-between text-sm">
      <span className="text-fg-muted">{label}</span>
      <span className={`font-mono ${pending ? 'text-fg-faint' : ''}`}>{value}</span>
    </div>
  )
}
