/**
 * Shared loading indicators for async UI (manifests, lists, inspector, etc.).
 */

export function LoadingSpinner({ size = 'md', className = '' }) {
  return (
    <span
      className={['klew-spinner', `klew-spinner-${size}`, className].filter(Boolean).join(' ')}
      aria-hidden="true"
    />
  )
}

/** Centered block with spinner + message. */
export function LoadingState({
  message = 'Loading…',
  compact = false,
  className = '',
}) {
  return (
    <div
      className={[
        'klew-loading-state',
        compact ? 'is-compact' : '',
        className,
      ].filter(Boolean).join(' ')}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <LoadingSpinner size={compact ? 'sm' : 'md'} />
      {message ? <span className="klew-loading-state-message">{message}</span> : null}
    </div>
  )
}

/** Inline spinner + label for toolbars, headers, and table rows. */
export function InlineLoading({ message, className = '' }) {
  return (
    <span
      className={['klew-loading-inline', className].filter(Boolean).join(' ')}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <LoadingSpinner size="sm" />
      {message ? <span>{message}</span> : null}
    </span>
  )
}
