import { Component, type ReactNode } from 'react'

type Props = { onError: () => void; children: ReactNode }

// React's own error boundary: a render error shows the fail-closed notice again and renders nothing.
export class ErrorBoundary extends Component<Props, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch() {
    this.props.onError()
  }

  render() {
    return this.state.failed ? null : this.props.children
  }
}
