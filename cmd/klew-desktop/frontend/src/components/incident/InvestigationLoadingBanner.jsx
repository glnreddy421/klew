import { LoadingSpinner } from '../LoadingSpinner.jsx'

/** Compact banner while correlation runs; resources catalog stays interactive. */
export function InvestigationLoadingBanner({ onOpenOverview }) {
  return (
    <div className="investigation-loading-banner" role="status" aria-live="polite" aria-busy="true">
      <span className="investigation-loading-banner-text">
        <LoadingSpinner size="sm" />
        Correlating cluster data in the background…
      </span>
      {onOpenOverview && (
        <button type="button" className="text-link-btn" onClick={onOpenOverview}>
          View overview
        </button>
      )}
    </div>
  )
}
