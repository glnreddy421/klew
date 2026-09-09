import { useEffect, useRef } from 'react'
import { formatFindResultSummary } from '../lib/terminalFind'

export function TerminalFindBar({
  open,
  query,
  resultIndex,
  resultCount,
  onQueryChange,
  onFindNext,
  onFindPrevious,
  onClose,
}) {
  const inputRef = useRef(null)

  useEffect(() => {
    if (!open) return undefined
    const frame = requestAnimationFrame(() => {
      inputRef.current?.focus()
      inputRef.current?.select()
    })
    return () => cancelAnimationFrame(frame)
  }, [open])

  if (!open) return null

  const summary = formatFindResultSummary(resultIndex, resultCount)

  return (
    <div
      className="terminal-find-bar"
      role="search"
      aria-label="Find in terminal"
      onMouseDown={(e) => e.stopPropagation()}
    >
      <input
        ref={inputRef}
        type="search"
        className="terminal-find-input"
        placeholder="Find in terminal…"
        value={query}
        spellCheck={false}
        autoComplete="off"
        autoCorrect="off"
        autoCapitalize="off"
        aria-label="Find text"
        onChange={(e) => onQueryChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            if (e.shiftKey) onFindPrevious()
            else onFindNext()
          }
          if (e.key === 'Escape') {
            e.preventDefault()
            onClose()
          }
        }}
      />
      <span className="terminal-find-summary" aria-live="polite">{summary}</span>
      <button
        type="button"
        className="terminal-find-btn"
        aria-label="Previous match"
        title="Previous (Shift+Enter)"
        disabled={!query.trim()}
        onClick={onFindPrevious}
      >
        ↑
      </button>
      <button
        type="button"
        className="terminal-find-btn"
        aria-label="Next match"
        title="Next (Enter)"
        disabled={!query.trim()}
        onClick={onFindNext}
      >
        ↓
      </button>
      <button
        type="button"
        className="terminal-find-btn terminal-find-close"
        aria-label="Close find"
        title="Close (Esc)"
        onClick={onClose}
      >
        ×
      </button>
    </div>
  )
}
