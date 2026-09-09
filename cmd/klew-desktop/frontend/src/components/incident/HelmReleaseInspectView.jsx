import { useEffect, useMemo, useState } from 'react'
import { StatusBadge } from './StatusBadge'
import { KindIcon } from '../KindIcon'
import { formatEntityAge } from '../../lib/entityTable'

function findSection(inspect, id) {
  for (const group of inspect?.groups || []) {
    for (const section of group.sections || []) {
      if (section.id === id) return section
    }
  }
  for (const section of inspect?.sections || []) {
    if (section.id === id) return section
  }
  return null
}

function summaryField(inspect, key) {
  return inspect?.summary?.find((f) => f.key === key)?.value
    || inspect?.status?.fields?.find((f) => (f.k || f.key) === key)?.v
    || inspect?.status?.fields?.find((f) => (f.k || f.key) === key)?.value
    || '—'
}

function NotesBlock({ lines }) {
  if (!lines?.length) return null
  return (
    <div className="helm-notes-body">
      {lines.map((line, idx) => {
        const warning = /warning|deprecated|must read|must be read|please read/i.test(line)
        return (
          <p
            key={`${idx}-${line.slice(0, 24)}`}
            className={warning ? 'helm-notes-line helm-notes-warning' : 'helm-notes-line'}
          >
            {warning && <span className="helm-notes-warn-icon" aria-hidden="true">⚠</span>}
            {line}
          </p>
        )
      })}
    </div>
  )
}

export function HelmReleaseInspectView({
  inspect,
  loading = false,
  error = null,
}) {
  const valuesSection = useMemo(() => findSection(inspect, 'helm-values'), [inspect])
  const notesSection = useMemo(() => findSection(inspect, 'helm-notes'), [inspect])

  const [userValuesOnly, setUserValuesOnly] = useState(true)

  useEffect(() => {
    setUserValuesOnly(true)
  }, [inspect?.key])

  const displayedValues = useMemo(() => {
    if (!valuesSection) return ''
    if (userValuesOnly && valuesSection.altCode) return valuesSection.altCode
    return valuesSection.code || ''
  }, [valuesSection, userValuesOnly])

  const updatedRaw = summaryField(inspect, 'Updated')
  const updatedLabel = updatedRaw && updatedRaw !== '—'
    ? formatEntityAge(updatedRaw)
    : '—'

  if (!inspect) {
    return <div className="inspect-empty muted">{loading ? 'Loading release…' : 'Select a release.'}</div>
  }

  return (
    <div className="helm-release-inspect">
      <header className="helm-release-header">
        <div className="helm-release-title-block">
          <div className="inspect-name-row">
            <KindIcon kind="HelmRelease" size={18} />
            <h4 className="inspect-name">
              <span className="inspect-name-text">{inspect.name}</span>
            </h4>
            {loading && <span className="muted inspect-loading">Loading…</span>}
          </div>
          <dl className="helm-release-meta-grid">
            <div className="helm-release-meta-item">
              <dt>Chart</dt>
              <dd>{summaryField(inspect, 'Chart')}</dd>
            </div>
            <div className="helm-release-meta-item">
              <dt>Updated</dt>
              <dd title={updatedRaw}>{updatedLabel}</dd>
            </div>
            <div className="helm-release-meta-item">
              <dt>Namespace</dt>
              <dd className="mono">{inspect.namespace || summaryField(inspect, 'Namespace') || '—'}</dd>
            </div>
            <div className="helm-release-meta-item">
              <dt>Version</dt>
              <dd>{summaryField(inspect, 'Version')}</dd>
            </div>
            <div className="helm-release-meta-item">
              <dt>Status</dt>
              <dd>
                <StatusBadge status={inspect.status?.tone} label={inspect.status?.label} />
              </dd>
            </div>
          </dl>
        </div>
      </header>

      {error && (
        <div className="inspect-fetch-error" role="alert">{error}</div>
      )}

      {valuesSection && (
        <section className="helm-release-section helm-values-section">
          <div className="helm-section-head">
            <h5>Values</h5>
            <label className="helm-values-toggle">
              <input
                type="checkbox"
                checked={userValuesOnly}
                onChange={(e) => setUserValuesOnly(e.target.checked)}
              />
              <span>{valuesSection.altToggleLabel || 'User-supplied values only'}</span>
            </label>
          </div>
          <pre className="helm-values-readonly mono" aria-readonly="true">{displayedValues}</pre>
        </section>
      )}

      {notesSection?.notes?.length > 0 && (
        <section className="helm-release-section helm-notes-section">
          <h5>Notes</h5>
          <NotesBlock lines={notesSection.notes} />
        </section>
      )}
    </div>
  )
}
