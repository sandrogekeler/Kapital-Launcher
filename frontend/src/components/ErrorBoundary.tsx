import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}
interface State {
  error: Error | null
}

/**
 * The app-level boundary. A render error shows its message in the data face
 * instead of a blank window; the component stack goes to the console, which a
 * dev build shows and a packaged build discards (forwarding it to the Go log
 * is on the roadmap, milestone 7).
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('render error', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="bg-canvas flex h-full items-center justify-center">
          <div className="p-8 text-center font-mono">
            <div className="text-danger mb-3 text-sm">render error</div>
            <div className="text-fg-faint max-w-md text-xs break-all select-text">
              {this.state.error.message}
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
