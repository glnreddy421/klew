import { useEffect, useState } from 'react'
import { KindIcon } from '../KindIcon'
import { LogoMark } from '../Logo'

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
      <OrbitStage idle={idle} />

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

function OrbitStage({ idle = false }) {
  const iconSize = idle ? 28 : 24
  return (
    <div className="collect-splash-stage" aria-hidden="true">
      <div className="collect-splash-ring collect-splash-ring-outer" />
      <div className="collect-splash-ring collect-splash-ring-mid" />
      <div className="collect-splash-ring collect-splash-ring-inner" />
      <div className="collect-splash-scan" />

      <div className="collect-splash-core">
        <LogoMark className="collect-splash-logo" />
      </div>

      <div className="collect-splash-orbit">
        {ORBIT_KINDS.map((kind, i) => (
          <span
            key={kind}
            className="collect-splash-sat"
            style={{ '--i': i, '--n': ORBIT_KINDS.length }}
          >
            <span className="collect-splash-sat-face">
              <KindIcon kind={kind} size={iconSize} />
            </span>
          </span>
        ))}
      </div>
    </div>
  )
}
