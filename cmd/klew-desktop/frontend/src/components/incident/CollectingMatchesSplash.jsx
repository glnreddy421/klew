import { useEffect, useState } from 'react'
import { KindIcon } from '../KindIcon'

const ORBIT_KINDS = ['Pod', 'Deployment', 'Service', 'Ingress', 'ConfigMap', 'Node']

const PHASES = [
  'Discovering workloads…',
  'Matching names & labels…',
  'Walking owner references…',
  'Mapping services & endpoints…',
  'Collecting matches…',
]

/**
 * Full-panel orbit splash.
 * - collecting: while investigation is still gathering matches
 * - idle: welcome / open state (same orbit, calmer copy)
 */
export function CollectingMatchesSplash({ variant = 'collecting' }) {
  const idle = variant === 'idle'
  const [phase, setPhase] = useState(0)

  useEffect(() => {
    if (idle) return undefined
    const id = setInterval(() => {
      setPhase((p) => (p + 1) % PHASES.length)
    }, 1600)
    return () => clearInterval(id)
  }, [idle])

  return (
    <div
      className={`collect-splash ${idle ? 'idle' : ''}`}
      role="status"
      aria-live="polite"
      aria-busy={idle ? undefined : true}
    >
      <OrbitStage />

      <div className="collect-splash-copy">
        {idle ? (
          <>
            <p className="collect-splash-title">Ready to investigate</p>
            <p className="collect-splash-phase">
              Pick context and namespace, then Investigate
            </p>
          </>
        ) : (
          <>
            <p className="collect-splash-title">Scanning cluster</p>
            <p className="collect-splash-phase" key={phase}>{PHASES[phase]}</p>
          </>
        )}
      </div>
    </div>
  )
}

function OrbitStage() {
  return (
    <div className="collect-splash-stage" aria-hidden="true">
      <div className="collect-splash-ring collect-splash-ring-outer" />
      <div className="collect-splash-ring collect-splash-ring-mid" />
      <div className="collect-splash-scan" />

      <div className="collect-splash-hex" aria-hidden="true">
        <svg viewBox="0 0 120 120" className="collect-splash-hex-svg">
          <polygon
            points="60,8 104,34 104,86 60,112 16,86 16,34"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          />
          <polygon
            points="60,28 88,44 88,76 60,92 32,76 32,44"
            fill="none"
            stroke="currentColor"
            strokeWidth="1"
            opacity="0.45"
          />
        </svg>
      </div>

      <div className="collect-splash-core">
        <KindIcon kind="Pod" size={36} />
      </div>

      <div className="collect-splash-orbit">
        {ORBIT_KINDS.map((kind, i) => (
          <span
            key={kind}
            className="collect-splash-sat"
            style={{ '--i': i, '--n': ORBIT_KINDS.length }}
          >
            <span className="collect-splash-sat-face">
              <KindIcon kind={kind} size={22} />
            </span>
          </span>
        ))}
      </div>
    </div>
  )
}
