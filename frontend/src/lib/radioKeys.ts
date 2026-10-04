import type { KeyboardEvent } from 'react'

const FORWARD = ['ArrowRight', 'ArrowDown']
const BACK = ['ArrowLeft', 'ArrowUp']

/**
 * Arrow keys inside a radio group move the focus between its enabled options,
 * wrapping at the ends; Enter and Space pick the focused one, as on any
 * button. The arrows only move: a choice that does something when picked
 * (switching a pack) must not do it on the way past. Put it on the group's
 * `onKeyDown`.
 */
export function moveRadioFocus(e: KeyboardEvent<HTMLElement>) {
  const step = FORWARD.includes(e.key) ? 1 : BACK.includes(e.key) ? -1 : 0
  if (step === 0) return
  const radios = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[role="radio"]')).filter(
    (r) => !(r as HTMLButtonElement).disabled,
  )
  const at = radios.findIndex((r) => r === document.activeElement)
  if (at < 0 || radios.length < 2) return
  e.preventDefault()
  radios[(at + step + radios.length) % radios.length]?.focus()
}
