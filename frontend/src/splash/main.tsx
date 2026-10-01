import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '../style.css'
import { SplashApp } from './SplashApp'
import { installBridge } from './bridge'

// Before the first render, so Go's first push finds the bridge in place.
installBridge()

const container = document.getElementById('root')
if (!container) throw new Error('splash.html has no #root')

createRoot(container).render(
  <StrictMode>
    <SplashApp />
  </StrictMode>,
)
