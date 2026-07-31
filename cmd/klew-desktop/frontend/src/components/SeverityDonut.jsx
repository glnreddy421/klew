import { useMemo } from 'react'

export const SEVERITY_ROWS = [
  { id: 'fatal', label: 'FATAL', tone: 'fatal' },
  { id: 'error', label: 'ERROR', tone: 'error' },
  { id: 'warn', label: 'WARN', tone: 'warn' },
  { id: 'info', label: 'INFO', tone: 'info' },
  { id: 'debug', label: 'DEBUG', tone: 'debug' },
  { id: 'trace', label: 'TRACE', tone: 'trace' },
]

/**
 * Donut + legend for log / event severity counts.
 */
export function SeverityCountsBody({
  severity = {},
  rows = SEVERITY_ROWS,
  label = 'Severity breakdown',
}) {
  const total = rows.reduce((n, r) => n + (severity[r.id] || 0), 0)

  return (
    <div className="gp-counts-body">
      <SeverityDonut rows={rows} severity={severity} total={total} label={label} />
      <ul className="gp-pie-legend">
        {rows.map((r) => {
          const count = severity[r.id] || 0
          const pct = total > 0 ? ((count / total) * 100).toFixed(1) : '0.0'
          return (
            <li key={r.id} className={`gp-pie-legend-row tone-${r.tone}`}>
              <span className="gp-pie-swatch" aria-hidden="true" />
              <span className="gp-pie-legend-label">{r.label}</span>
              <span className="gp-pie-legend-count mono">{count}</span>
              <span className="gp-pie-legend-pct mono muted">{pct}%</span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

export function SeverityDonut({ rows, severity, total, label = 'Severity breakdown' }) {
  const size = 120
  const cx = size / 2
  const cy = size / 2
  const outerR = size / 2 - 2
  const innerR = outerR * 0.58

  const slices = useMemo(() => {
    let angle = 0
    return rows
      .map((row) => {
        const count = severity[row.id] || 0
        const sweep = total > 0 ? (count / total) * 360 : 0
        const start = angle
        angle += sweep
        return {
          ...row,
          count,
          pct: total > 0 ? (count / total) * 100 : 0,
          start,
          end: angle,
        }
      })
      .filter((slice) => slice.count > 0)
  }, [rows, severity, total])

  return (
    <div className="gp-pie-wrap">
      <svg
        className="gp-pie"
        viewBox={`0 0 ${size} ${size}`}
        role="img"
        aria-label={`${label}, ${total} total`}
      >
        {total === 0 ? (
          <circle cx={cx} cy={cy} r={(outerR + innerR) / 2} className="gp-pie-empty" />
        ) : (
          slices.map((slice) => (
            <path
              key={slice.id}
              d={donutArc(cx, cy, outerR, innerR, slice.start, slice.end)}
              className={`gp-pie-slice tone-${slice.tone}`}
            >
              <title>{`${slice.label}: ${slice.count} (${slice.pct.toFixed(1)}%)`}</title>
            </path>
          ))
        )}
        <text
          x={cx}
          y={cy}
          className="gp-pie-center mono"
          textAnchor="middle"
          dominantBaseline="middle"
        >
          {fmtCount(total)}
        </text>
      </svg>
    </div>
  )
}

function fmtCount(n) {
  const x = Number(n) || 0
  if (x >= 1_000_000) return `${(x / 1_000_000).toFixed(1)}M`
  if (x >= 10_000) return `${Math.round(x / 1000)}k`
  if (x >= 1000) return `${(x / 1000).toFixed(1)}k`
  return String(x)
}

function polar(cx, cy, r, deg) {
  const rad = ((deg - 90) * Math.PI) / 180
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) }
}

function donutArc(cx, cy, outerR, innerR, startAngle, endAngle) {
  if (endAngle - startAngle >= 359.99) {
    return [
      `M ${cx} ${cy - outerR}`,
      `A ${outerR} ${outerR} 0 1 1 ${cx - 0.01} ${cy - outerR}`,
      `L ${cx - 0.01} ${cy - innerR}`,
      `A ${innerR} ${innerR} 0 1 0 ${cx} ${cy - innerR}`,
      'Z',
    ].join(' ')
  }

  const largeArc = endAngle - startAngle > 180 ? 1 : 0
  const outerStart = polar(cx, cy, outerR, startAngle)
  const outerEnd = polar(cx, cy, outerR, endAngle)
  const innerEnd = polar(cx, cy, innerR, endAngle)
  const innerStart = polar(cx, cy, innerR, startAngle)

  return [
    `M ${outerStart.x} ${outerStart.y}`,
    `A ${outerR} ${outerR} 0 ${largeArc} 1 ${outerEnd.x} ${outerEnd.y}`,
    `L ${innerEnd.x} ${innerEnd.y}`,
    `A ${innerR} ${innerR} 0 ${largeArc} 0 ${innerStart.x} ${innerStart.y}`,
    'Z',
  ].join(' ')
}
