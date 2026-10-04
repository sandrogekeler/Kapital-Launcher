import { useId, useLayoutEffect, useRef } from 'react'
import { Hint } from './Notes'
import { SubHeading } from './Section'

/** A value worth one click, marked on the track where it falls: the pack's recommendation. */
export interface RangeMark {
  value: number
  /** What the mark is, before its value: "Recommended". */
  label: string
  /** The value in words, as the readout writes it. */
  valueLabel: string
}

interface Props {
  /** The block's heading, which also names the slider. */
  label: string
  /** The value in words, at the right of the heading. */
  valueLabel: string
  /** Faint words after the value: what it is out of. */
  valueNote?: string
  value: number
  min: number
  max: number
  step: number
  onChange: (value: number) => void
  hint?: string
  /** Drawn under the track at its value, and sets it on a click; left out when it is off the track. */
  mark?: RangeMark | null
}

const share = (value: number, min: number, max: number) =>
  max > min ? Math.min(1, Math.max(0, (value - min) / (max - min))) : 0

/**
 * A slider with a heading, its value in the data face and a line under it.
 * Its look is `.range` in style.css, drawn from tokens; the filled part of the
 * track follows the value through the `--range-fill` property, set on the
 * element and not through `style`, as the loading card's bar does. A mark sits
 * under the track where its value falls (`.range-mark`, placed by `--mark-at`
 * the same way), pressed while the slider is on it (issue 189).
 */
export function RangeField({
  label,
  valueLabel,
  valueNote,
  value,
  min,
  max,
  step,
  onChange,
  hint,
  mark,
}: Props) {
  const id = useId()
  const input = useRef<HTMLInputElement>(null)
  const markRef = useRef<HTMLButtonElement>(null)
  const shown = mark && mark.value >= min && mark.value <= max ? mark : null
  useLayoutEffect(() => {
    input.current?.style.setProperty('--range-fill', `${share(value, min, max) * 100}%`)
  }, [value, min, max])
  useLayoutEffect(() => {
    if (shown) markRef.current?.style.setProperty('--mark-at', String(share(shown.value, min, max)))
  }, [shown, min, max])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-4">
        <SubHeading id={id}>{label}</SubHeading>
        <span className="flex items-baseline gap-2">
          <span className="text-md font-mono">{valueLabel}</span>
          {valueNote && <span className="text-fg-muted text-xs">{valueNote}</span>}
        </span>
      </div>
      <input
        ref={input}
        type="range"
        aria-labelledby={id}
        aria-valuetext={valueLabel}
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="range"
      />
      {shown && (
        <div className="relative h-7">
          <button
            ref={markRef}
            type="button"
            aria-pressed={value === shown.value}
            onClick={() => onChange(shown.value)}
            className={`range-mark rounded-pill duration-fast ease-standard absolute top-0 flex cursor-pointer items-baseline gap-1.5 border px-2.5 py-1 text-xs whitespace-nowrap transition-colors ${
              value === shown.value
                ? 'bg-accent-wash border-accent-edge text-fg'
                : 'border-line text-fg-muted hover:border-line-strong hover:text-fg'
            }`}
          >
            {shown.label}
            <span className="text-fg font-mono font-medium">{shown.valueLabel}</span>
          </button>
        </div>
      )}
      {hint && <Hint>{hint}</Hint>}
    </div>
  )
}
