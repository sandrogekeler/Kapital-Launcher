import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { EngineCard } from './EngineCard'

describe('EngineCard', () => {
  afterEach(() => {
    cleanup()
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS })
  })

  it('says the account is a Microsoft one through Prism, until the player plays offline', () => {
    useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: 'Sandro_G' } })
    const { rerender } = render(<EngineCard />)
    expect(screen.getByText('Microsoft account · via Prism')).toBeInTheDocument()
    useSettingsStore.setState({
      settings: {
        ...DEFAULT_SETTINGS,
        profileName: 'Sandro_G',
        offline: true,
        offlineName: 'Steve',
      },
    })
    rerender(<EngineCard />)
    expect(screen.getByText('Offline')).toBeInTheDocument()
    expect(screen.queryByText('Microsoft account · via Prism')).toBeNull()
  })
})
