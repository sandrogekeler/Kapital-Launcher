import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import { installErrorReporting } from './lib/reportError'
import { BUNDLED_ART } from './lib/art'
import { preloadImages } from './lib/useLoadedImage'

installErrorReporting()
preloadImages(BUNDLED_ART)

const container = document.getElementById('root')
if (!container) throw new Error('index.html has no #root')

createRoot(container).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>,
)
