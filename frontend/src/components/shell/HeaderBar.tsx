import { useEffect, useState } from 'react'
import { hasWailsBridge, readOr } from '../../lib/ipc'
import { Copy, Minus, Settings, Square, X } from '../../lib/icons'
import type { LucideIcon } from '../../lib/icons'
import {
  Quit,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from '../../../wailsjs/runtime/runtime'
import { Brand } from './Brand'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'

interface Props {
  /** Wails' `Environment().platform`: "windows", "darwin" or "linux". */
  platform: string
  onOpenSettings?: () => void
}

/**
 * The window's own header bar, replacing the OS title bar
 * (docs/adr/0009-window-chrome.md). Its empty area drags the window and a
 * double-click maximises; anything interactive opts out of the drag.
 *
 * macOS keeps its native traffic lights at the top left, so the brand starts
 * after them and no window buttons are drawn. Everywhere else the bar ends in
 * our own minimise, maximise and close.
 */
export function HeaderBar({ platform, onOpenSettings }: Props) {
  const mac = platform === 'darwin'
  return (
    <header
      onDoubleClick={() => bridged(WindowToggleMaximise)}
      className={`bg-sunken border-line flex h-(--layout-titlebar) shrink-0 items-center border-b [--wails-draggable:drag] ${mac ? 'pl-20' : 'pl-5'}`}
    >
      <Brand />
      <div className="grow" />
      <div
        onDoubleClick={(e) => e.stopPropagation()}
        className="flex h-full items-center gap-1 [--wails-draggable:no-drag]"
      >
        <IconButton
          icon={Settings}
          title="Settings"
          size="sm"
          onClick={() => onOpenSettings?.()}
          disabled={!onOpenSettings}
        />
        {mac ? <div className="w-2" /> : <WindowControls />}
      </div>
    </header>
  )
}

/** Minimise, maximise or restore, and close, sized and ordered as Windows draws them. */
function WindowControls() {
  const [maximised, setMaximised] = useState(false)

  // The restore glyph follows the window, however it got maximised: our
  // button, a double-click, Win+Up or a drag to the top edge. Each of those
  // resizes the page, so resize is the moment to ask.
  useEffect(() => {
    let live = true
    const sync = async () => {
      const m = await readOr(WindowIsMaximised, false)
      if (live) setMaximised(m)
    }
    void sync()
    window.addEventListener('resize', sync)
    return () => {
      live = false
      window.removeEventListener('resize', sync)
    }
  }, [])

  return (
    <div className="ml-1 flex h-full">
      <WindowButton icon={Minus} title="Minimise" onClick={() => bridged(WindowMinimise)} />
      <WindowButton
        icon={maximised ? Copy : Square}
        title={maximised ? 'Restore' : 'Maximise'}
        onClick={() => bridged(WindowToggleMaximise)}
      />
      <WindowButton icon={X} title="Close" onClick={() => bridged(Quit)} danger />
    </div>
  )
}

function WindowButton({
  icon,
  title,
  onClick,
  danger,
}: {
  icon: LucideIcon
  title: string
  onClick: () => void
  danger?: boolean
}) {
  const hover = danger ? 'hover:bg-danger hover:text-canvas' : 'hover:bg-hover hover:text-fg'
  return (
    <button
      type="button"
      onClick={onClick}
      title={title}
      aria-label={title}
      className={`text-fg-muted duration-fast ease-standard flex h-full w-11.5 cursor-pointer items-center justify-center transition-colors ${hover}`}
    >
      <Icon icon={icon} size="sm" />
    </button>
  )
}

/**
 * Window calls are fire-and-forget and throw synchronously without a Wails
 * bridge. In the browser-only preview there is no window to move, so they do
 * nothing there.
 */
function bridged(call: () => void) {
  if (hasWailsBridge()) call()
}
