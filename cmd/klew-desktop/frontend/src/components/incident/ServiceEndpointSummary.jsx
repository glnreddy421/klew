import { splitEndpointParts } from '../../lib/serviceEndpoints'

export function ServiceEndpointSummary({ name, endpoints, parts: partsProp }) {
  const parts = partsProp || splitEndpointParts(endpoints)
  if (!parts.length) return null

  return (
    <section className="inspect-section inspect-section-card service-endpoint-summary">
      <h5 className="inspect-section-label">Endpoint</h5>
      <dl className="inspect-prop-list">
        {name && (
          <div className="inspect-prop-row">
            <dt>Name</dt>
            <dd className="mono">{name}</dd>
          </div>
        )}
        <div className="inspect-prop-row service-endpoint-row">
          <dt>Endpoints</dt>
          <dd>
            <ul className="service-endpoint-list">
              {parts.map((part) => (
                <li key={part} className="service-endpoint-chip mono">{part}</li>
              ))}
            </ul>
          </dd>
        </div>
      </dl>
    </section>
  )
}
