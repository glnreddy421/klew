import { useMemo, useState } from 'react'
import { resolveInspectRef } from '../../lib/inspectEnrich'
import { containerStateTone, parseInspectContainers } from '../../lib/inspectContainers'
import { decodeSecretValue } from '../../lib/secretDisplay'

export function InspectContainersPanel({ sections = [], inspectNamespace = '', onInspect }) {
  const model = useMemo(() => parseInspectContainers(sections), [sections])
  const { containers, initContainers, podLevel } = model

  if (!containers.length && !initContainers.length) {
    return (
      <div className="inspect-containers-panel">
        {podLevel.fallback.map((s) => (
          <FallbackSection key={s.id || s.title} section={s} />
        ))}
      </div>
    )
  }

  return (
    <div className="inspect-containers-panel">
      {containers.map((c) => (
        <ContainerCard
          key={c.name}
          container={c}
          onInspect={onInspect}
          inspectNamespace={inspectNamespace}
        />
      ))}

      {initContainers.length > 0 && (
        <section className="inspect-containers-group">
          <header className="inspect-containers-group-head">
            <SubIcon kind="init" />
            <h6>Init containers</h6>
            <span className="inspect-containers-group-count">{initContainers.length}</span>
          </header>
          <div className="inspect-containers-group-body">
            {initContainers.map((c) => (
              <ContainerCard
                key={`init-${c.name}`}
                container={c}
                compact
                onInspect={onInspect}
                inspectNamespace={inspectNamespace}
              />
            ))}
          </div>
        </section>
      )}

      {podLevel.restartHistory.length > 0 && (
        <PodMetaPanel icon="history" title="Restart history">
          <div className="inspect-kv-grid">
            {podLevel.restartHistory.map((row) => (
              <div key={row.Name} className="inspect-kv-card">
                <span className="inspect-kv-card-label mono">{row.Name}</span>
                <span className="inspect-kv-card-value">
                  {row.Restarts} restarts · {row['Last State']}
                  {row['Last Reason'] ? ` (${row['Last Reason']})` : ''}
                </span>
              </div>
            ))}
          </div>
        </PodMetaPanel>
      )}

      {podLevel.imagePullSecrets.length > 0 && (
        <PodMetaPanel icon="secret" title="Image pull secrets">
          <ul className="inspect-chip-links">
            {podLevel.imagePullSecrets.map((name) => (
              <RefChip
                key={name}
                value={name}
                section={{ id: 'imagePullSecrets', title: 'Image Pull Secrets' }}
                columnName="Name"
                inspectNamespace={inspectNamespace}
                onInspect={onInspect}
              />
            ))}
          </ul>
        </PodMetaPanel>
      )}

      {podLevel.sidecars.length > 0 && (
        <PodMetaPanel icon="sidecar" title="Sidecars">
          <div className="inspect-kv-grid">
            {podLevel.sidecars.map((row) => (
              <div key={row.Name} className="inspect-kv-card">
                <span className="inspect-kv-card-label mono">{row.Name}</span>
                <span className="inspect-kv-card-value muted">{row.Image}</span>
              </div>
            ))}
          </div>
        </PodMetaPanel>
      )}

      {podLevel.fallback.map((s) => (
        <FallbackSection key={s.id || s.title} section={s} />
      ))}
    </div>
  )
}

function ContainerCard({ container, compact = false, onInspect, inspectNamespace }) {
  const tone = containerStateTone(container.state?.state)
  const [open, setOpen] = useState(!compact)

  const stats = [
    container.resources?.reqCpu && { label: 'CPU req', value: container.resources.reqCpu },
    container.resources?.limCpu && { label: 'CPU lim', value: container.resources.limCpu },
    container.resources?.reqMem && { label: 'Mem req', value: container.resources.reqMem },
    container.resources?.limMem && { label: 'Mem lim', value: container.resources.limMem },
    container.state?.allocCpu && { label: 'Alloc CPU', value: container.state.allocCpu },
    container.state?.allocMem && { label: 'Alloc mem', value: container.state.allocMem },
  ].filter(Boolean)

  return (
    <article className={`inspect-container-card tone-${tone} ${compact ? 'is-compact' : ''}`}>
      <header className="inspect-container-card-head">
        <button
          type="button"
          className="inspect-container-card-toggle"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
        >
          <span className={`inspect-container-led tone-${tone}`} aria-hidden="true" />
          <div className="inspect-container-card-title">
            <span className="inspect-container-name mono">{container.name}</span>
            {container.init && <span className="inspect-container-tag">init</span>}
            <StateBadge state={container.state?.state} reason={container.state?.reason} ready={container.state?.ready} />
          </div>
          <Chevron open={open} />
        </button>
        {container.spec?.image && (
          <p className="inspect-container-image mono" title={container.spec.image}>
            {container.spec.image}
          </p>
        )}
        {stats.length > 0 && (
          <div className="inspect-container-stat-row">
            {stats.map((s) => (
              <span key={s.label} className="inspect-container-stat">
                <span className="inspect-container-stat-k">{s.label}</span>
                <span className="inspect-container-stat-v mono">{s.value}</span>
              </span>
            ))}
            {container.resources?.ports && (
              <span className="inspect-container-stat">
                <span className="inspect-container-stat-k">Ports</span>
                <span className="inspect-container-stat-v mono">{container.resources.ports}</span>
              </span>
            )}
          </div>
        )}
      </header>

      {open && (
        <div className="inspect-container-card-body">
          {(container.state?.state || container.state?.restarts != null) && (
            <SubPanel icon="runtime" title="Runtime">
              <PropGrid rows={[
                ['Ready', container.state.ready],
                ['State', container.state.state],
                ['Reason', container.state.reason],
                ['Restarts', container.state.restarts],
                ['Exit code', container.state.exit],
                ['Started', container.state.started],
                ['Image', container.state.image || container.spec.image],
                ['Image ID', container.state.imageId],
                ['Container ID', container.state.containerId],
              ]} />
            </SubPanel>
          )}

          {hasContainerSpec(container) && (
            <SubPanel icon="spec" title="Spec">
              <PropGrid rows={[
                ['Image', container.spec.image],
                ['Pull policy', container.spec.pullPolicy],
                ['Ports', container.resources?.ports],
                ['Command', container.spec.command],
                ['Args', container.spec.args],
                ['Working dir', container.spec.workingDir],
              ]} />
            </SubPanel>
          )}

          {(container.mounts.length > 0 || container.statusMounts.length > 0) && (
            <SubPanel icon="mount" title="Volume mounts">
              {container.mounts.length > 0 && (
                <MountTable title="Spec" rows={container.mounts} />
              )}
              {container.statusMounts.length > 0 && (
                <MountTable title="Status" rows={container.statusMounts.map((m) => ({
                  volume: m.volume,
                  mountPath: m.mountPath,
                  readOnly: m.readOnly,
                  subPath: m.recursiveRO ? `recursive: ${m.recursiveRO}` : '',
                }))} />
              )}
            </SubPanel>
          )}

          {container.probes.length > 0 && (
            <SubPanel icon="probe" title="Health checks">
              <div className="inspect-mini-table-wrap">
                <table className="inspect-mini-table">
                  <thead>
                    <tr>
                      <th>Probe</th>
                      <th>Type</th>
                      <th>Target</th>
                      <th>Delay</th>
                      <th>Period</th>
                    </tr>
                  </thead>
                  <tbody>
                    {container.probes.map((p) => (
                      <tr key={`${p.type}-${p.probeKind}`}>
                        <td>{p.type}</td>
                        <td>{p.probeKind}</td>
                        <td className="mono">{p.target}</td>
                        <td>{p.initialDelay || '—'}</td>
                        <td>{p.period || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </SubPanel>
          )}

          {Object.values(container.security).some(Boolean) && (
            <SubPanel icon="lock" title="Security">
              <PropGrid rows={[
                ['Run as user', container.security.runAsUser],
                ['Run as group', container.security.runAsGroup],
                ['Privileged', container.security.privileged],
                ['Read-only root FS', container.security.readOnlyRootFs],
              ]} />
            </SubPanel>
          )}

          {container.env.length > 0 && (
            <SubPanel icon="env" title="Environment">
              <div className="inspect-env-list">
                {container.env.map((e) => (
                  <div key={`${e.name}-${e.source}`} className="inspect-env-row">
                    <span className="inspect-env-name mono">{e.name}</span>
                    <RefChip
                      value={e.source}
                      section={{ id: 'environment', title: 'Environment' }}
                      columnName="Source"
                      inspectNamespace={inspectNamespace}
                      onInspect={onInspect}
                      muted
                    />
                    {e.value && e.value !== e.source && (
                      <span className="inspect-env-value mono" title={e.value}>{e.value}</span>
                    )}
                  </div>
                ))}
              </div>
            </SubPanel>
          )}

          {container.secretEnv.length > 0 && (
            <SubPanel icon="secret" title="Secret values">
              {container.secretEnv.map((e) => (
                <SecretEnvRow key={`${e.name}-${e.secretKey}`} entry={e} />
              ))}
            </SubPanel>
          )}
        </div>
      )}
    </article>
  )
}

const LONG_PROP_KEYS = new Set([
  'Image',
  'Image ID',
  'Container ID',
  'Command',
  'Args',
])

function hasContainerSpec(container) {
  const s = container?.spec || {}
  const ports = container?.resources?.ports
  return Boolean(
    s.image || s.pullPolicy || s.command || s.args || s.workingDir || ports,
  )
}

function PropGrid({ rows }) {
  const items = rows.filter(([, v]) => v != null && String(v).trim() !== '' && v !== '—')
  if (!items.length) return null
  return (
    <dl className="inspect-container-props">
      {items.map(([k, v]) => (
        <div key={k} className="inspect-container-prop">
          <dt>{k}</dt>
          <dd
            className={[
              'mono',
              LONG_PROP_KEYS.has(k) ? 'inspect-container-prop-long' : '',
            ].filter(Boolean).join(' ')}
            title={String(v)}
          >
            {v}
          </dd>
        </div>
      ))}
    </dl>
  )
}

function MountTable({ title, rows }) {
  if (!rows.length) return null
  return (
    <div className="inspect-mount-block">
      {title && <div className="inspect-mount-block-label">{title}</div>}
      <div className="inspect-mini-table-wrap">
        <table className="inspect-mini-table">
          <thead>
            <tr>
              <th>Volume</th>
              <th>Mount path</th>
              <th>Sub path</th>
              <th>RO</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((m, i) => (
              <tr key={`${m.volume}-${i}`}>
                <td className="mono">{m.volume}</td>
                <td className="mono">{m.mountPath}</td>
                <td className="mono">{m.subPath || '—'}</td>
                <td>{m.readOnly || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function StateBadge({ state, reason, ready }) {
  const tone = containerStateTone(state)
  const label = reason ? `${state} · ${reason}` : (state || (ready === 'true' ? 'ready' : 'unknown'))
  return (
    <span className={`inspect-container-state-badge tone-${tone}`}>
      {label}
    </span>
  )
}

function RefChip({ value, section, columnName, inspectNamespace, onInspect, muted = false }) {
  const ref = resolveInspectRef(value, {
    columnName,
    section,
    groupId: 'containers',
    inspectNamespace,
  })
  if (ref?.key && onInspect) {
    return (
      <button
        type="button"
        className="inspect-ref-chip"
        onClick={() => onInspect(ref.key)}
        title={`Inspect ${ref.kind}/${ref.name}`}
      >
        {value}
      </button>
    )
  }
  return <span className={muted ? 'muted mono' : 'mono'}>{value}</span>
}

function SecretEnvRow({ entry }) {
  const [revealed, setRevealed] = useState(false)
  const display = revealed ? decodeSecretValue(entry.value) : '••••••'
  return (
    <div className="inspect-secret-env-row">
      <span className="mono">{entry.name}</span>
      <span className="muted">{entry.secretKey}</span>
      <span className="inspect-secret-env-value mono">{display}</span>
      <button type="button" className="inspect-reveal-btn" onClick={() => setRevealed((v) => !v)}>
        {revealed ? 'Hide' : 'Reveal'}
      </button>
    </div>
  )
}

function SubPanel({ icon, title, children }) {
  return (
    <section className="inspect-container-subpanel">
      <header className="inspect-container-subpanel-head">
        <SubIcon kind={icon} />
        <h6>{title}</h6>
      </header>
      <div className="inspect-container-subpanel-body">{children}</div>
    </section>
  )
}

function PodMetaPanel({ icon, title, children }) {
  return (
    <section className="inspect-container-meta">
      <header className="inspect-container-meta-head">
        <SubIcon kind={icon} />
        <h6>{title}</h6>
      </header>
      {children}
    </section>
  )
}

function Chevron({ open }) {
  return (
    <svg className={`inspect-container-chevron ${open ? 'is-open' : ''}`} viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
      <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </svg>
  )
}

function FallbackSection({ section }) {
  if (!section?.table?.rows?.length && !section?.fields?.length) return null
  return (
    <section className="inspect-section-card">
      <h5 className="inspect-section-label">{section.title}</h5>
      {section.fields?.length > 0 && (
        <dl className="inspect-container-props">
          {section.fields.map((f) => (
            <div key={f.key} className="inspect-container-prop">
              <dt>{f.key}</dt>
              <dd>{f.value}</dd>
            </div>
          ))}
        </dl>
      )}
      {section.table?.rows?.length > 0 && (
        <div className="inspect-mini-table-wrap">
          <table className="inspect-mini-table">
            <thead>
              <tr>
                {section.table.columns.map((c) => <th key={c}>{c}</th>)}
              </tr>
            </thead>
            <tbody>
              {section.table.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => <td key={j}>{cell || '—'}</td>)}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

function SubIcon({ kind }) {
  const icons = {
    runtime: <path d="M4 12V4l4-2 4 2v8l-4 2-4-2z" />,
    spec: <path d="M5 4h6M5 8h4M5 12h6" />,
    mount: <path d="M3 6h10v8H3zM6 6V4h4v2" />,
    probe: <path d="M8 3v3M5 10a3 3 0 006 0" />,
    lock: <path d="M5 8V6a3 3 0 016 0v2M4 8h8v6H4z" />,
    env: <path d="M4 6h8M4 10h5M4 14h8" />,
    secret: <path d="M8 3l4 2v6l-4 2-4-2V5z" />,
    init: <path d="M8 4v8M4 8h8" />,
    history: <path d="M8 4a4 4 0 100 8M8 8l2 2" />,
    sidecar: <path d="M3 8h10M8 3v10" />,
  }
  return (
    <span className="inspect-container-subicon" aria-hidden="true">
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round">
        {icons[kind] || icons.spec}
      </svg>
    </span>
  )
}
