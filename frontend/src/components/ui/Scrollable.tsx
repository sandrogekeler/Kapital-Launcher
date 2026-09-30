import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { ElementType, PointerEvent as ReactPointerEvent, ReactNode } from 'react'

interface Props {
  children: ReactNode
  /** The element that scrolls: `main` for the chapter view, `section` for a panel. */
  as?: ElementType
  /** Classes for the scrolling element; the wrapper only positions the thumb. */
  className?: string
  'aria-label'?: string
}

/** The thumb never shrinks below this, in px, so it stays grabbable on a long page. */
const MIN_THUMB = 32

/**
 * A scroll container whose scrollbar is a pill drawn over the content (#56).
 * The native bar is hidden app-wide (style.css) because it takes a strip
 * beside the content and draws its own track and buttons; this thumb follows
 * the element's own scrolling and drags it back, and nothing else changes:
 * the wheel, the keys and touch still scroll the element itself.
 */
export function Scrollable({ children, as: Tag = 'div', className, ...rest }: Props) {
  const ref = useRef<HTMLElement>(null)
  const [thumb, setThumb] = useState<{ top: number; height: number } | null>(null)

  const measure = useCallback(() => {
    const el = ref.current
    if (!el) return
    const { scrollTop, scrollHeight, clientHeight } = el
    if (scrollHeight <= clientHeight + 1) {
      setThumb(null)
      return
    }
    const height = Math.max(MIN_THUMB, (clientHeight * clientHeight) / scrollHeight)
    const top = (scrollTop / (scrollHeight - clientHeight)) * (clientHeight - height)
    setThumb({ top, height })
  }, [])

  // Before paint, so the thumb is never seen jumping into place; then on
  // every scroll and whenever the element or its content changes size.
  useLayoutEffect(measure, [measure, children])
  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.addEventListener('scroll', measure, { passive: true })
    // jsdom has no ResizeObserver; the tests measure through scroll events.
    const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure)
    ro?.observe(el)
    if (el.firstElementChild) ro?.observe(el.firstElementChild)
    return () => {
      el.removeEventListener('scroll', measure)
      ro?.disconnect()
    }
  }, [measure])

  // Dragging the thumb scrolls the element by the same proportion.
  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>) => {
    const el = ref.current
    if (!el || !thumb) return
    e.preventDefault()
    const startY = e.clientY
    const startTop = el.scrollTop
    const ratio = (el.scrollHeight - el.clientHeight) / (el.clientHeight - thumb.height)
    const onMove = (ev: PointerEvent) => {
      el.scrollTop = startTop + (ev.clientY - startY) * ratio
    }
    const onUp = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  return (
    <div className="relative flex min-h-0 min-w-0 grow">
      <Tag
        ref={ref}
        className={`min-h-0 min-w-0 grow overflow-y-auto ${className ?? ''}`}
        {...rest}
      >
        {children}
      </Tag>
      {thumb && (
        <div
          role="scrollbar"
          aria-orientation="vertical"
          aria-valuenow={Math.round(thumb.top)}
          onPointerDown={onPointerDown}
          // The thumb's place is measured from the element and changes on
          // every scroll: the one computed value the style rule allows.
          // eslint-disable-next-line no-restricted-syntax
          style={{ top: thumb.top, height: thumb.height }}
          className="bg-line-strong rounded-pill hover:bg-fg-faint duration-fast ease-standard absolute right-1 w-(--layout-scrollbar) cursor-default transition-colors"
        />
      )}
    </div>
  )
}
