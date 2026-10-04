import { useEffect } from 'react'

/**
 * Escape closes the panel that calls this. A field with an unsaved draft takes
 * the key first and stops it (TextField), so one press never loses a save.
 */
export function useEscapeClose(onClose: () => void) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
}
