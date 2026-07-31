import React from 'react'

/** Catches render crashes so the desktop window doesn't go blank. */
export class ErrorBoundary extends React.Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error) {
    return { error }
  }

  componentDidCatch(error, info) {
    console.error('UI crash:', error, info?.componentStack)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="ui-crash" role="alert">
          <h2>Something went wrong</h2>
          <p className="muted">{String(this.state.error?.message || this.state.error)}</p>
          <button
            type="button"
            className="btn btn-outline"
            onClick={() => this.setState({ error: null })}
          >
            Try again
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
