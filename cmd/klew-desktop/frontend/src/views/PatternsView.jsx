import { useState } from 'react'
import { getState } from '../lib/investigationViews'
import { StreamPatternsPanel } from '../components/StreamPatternsPanel'
import { EvidenceBoardTeaser } from '../components/evidence/EvidenceBoardPanel'
import { TimelineView } from './TimelineView'

const KINDS = [
  { id: 'logs', label: 'Log Patterns', hint: 'Templates and top words from container logs' },
  { id: 'events', label: 'Event Patterns', hint: 'Templates from Pod, Node, and PVC events' },
]

/**
 * Dedicated Patterns page — Log Patterns | Event Patterns.
 */
export function PatternsView({ view, running = false, onFilterLogs, onOpenEvidence }) {
  const [kind, setKind] = useState('logs')
  const state = getState(view)
  const patterns = view?.logPatterns || state.logPatterns || null
  const evidence = view?.evidence || state.liveEvidence || []

  return (
    <div className="inv-page patterns-page">
      <div className="patterns-kind-tabs" role="tablist" aria-label="Pattern kind">
        {KINDS.map((k) => (
          <button
            key={k.id}
            type="button"
            role="tab"
            aria-selected={kind === k.id}
            className={`patterns-kind-pill ${kind === k.id ? 'active' : ''}`}
            title={k.hint}
            onClick={() => setKind(k.id)}
          >
            {k.label}
          </button>
        ))}
      </div>
      <EvidenceBoardTeaser board={patterns?.evidenceBoard} onOpen={onOpenEvidence} />
      <p className="patterns-kind-hint muted">
        {kind === 'logs'
          ? 'Templates from container logs. Click a pattern to expand samples; click a word or field to filter Live tail.'
          : 'Templates from Pod, Node, and PVC Kubernetes events.'}
      </p>

      <div className="patterns-kind-body">
        {kind === 'logs' ? (
          <StreamPatternsPanel
            patterns={patterns}
            evidence={evidence}
            running={running}
            selectedPods={[]}
            onFilterLogs={onFilterLogs}
          />
        ) : (
          <TimelineView view={view} embedded />
        )}
      </div>
    </div>
  )
}
