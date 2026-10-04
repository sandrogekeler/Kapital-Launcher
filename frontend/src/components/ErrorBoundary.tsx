import { Component, type ErrorInfo, type ReactNode } from 'react'
import { reportError } from '../lib/reportError'

interface Props {
  children: ReactNode
}
interface State {
  error: Error | null
}

/**
 * The app-level boundary. A render error shows its message in the data face
 * instead of a blank window, and the error with its component stack goes to
 * the Go log, since a packaged build has no console.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    reportError('render', error, info.componentStack ?? undefined)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="bg-canvas flex h-full items-center justify-center">
          <div className="p-8 text-center font-mono">
            <div className="text-danger mb-3 text-sm">render error</div>
            <div className="text-fg-muted max-w-md text-xs break-all select-text">
              {this.state.error.message}
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
