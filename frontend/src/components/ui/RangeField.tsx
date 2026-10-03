import { useId, useLayoutEffect, useRef } from 'react'
import { Hint } from './Notes'
import { SubHeading } from './Section'

interface Props {
  /** The block's heading, which also names the slider. */
  label: string
  /** The value in words, at the right of the heading. */
  valueLabel: string
  value: number
  min: number
  max: number
  step: number
  onChange: (value: number) => void
  hint?: string
}

/**
 * A slider with a heading, its value in the data face and a line under it.
 * Its look is `.range` in style.css, drawn from tokens; the filled part of the
 * track follows the value through the `--range-fill` property, set on the
 * element and not through `style`, as the loading card's bar does.
 */
export function RangeField({ label, valueLabel, value, min, max, step, onChange, hint }: Props) {
  const id = useId()
  const input = useRef<HTMLInputElement>(null)
  useLayoutEffect(() => {
    const share = max > min ? ((value - min) / (max - min)) * 100 : 0
    input.current?.style.setProperty('--range-fill', `${share}%`)
  }, [value, min, max])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between">
        <SubHeading id={id}>{label}</SubHeading>
        <span className="font-mono text-sm">{valueLabel}</span>
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
      {hint && <Hint>{hint}</Hint>}
    </div>
  )
}
