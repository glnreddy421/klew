import { memo, useDeferredValue, useMemo } from 'react'
import { useResourceManifest } from '../../hooks/useResourceManifest.js'
import { isRbacForbiddenMessage } from '../../lib/rbacAccess.js'
import {
  getYamlRenderMeta,
  YAML_HIGHLIGHT_MAX_LINES,
} from '../../lib/yamlRender.js'
import { InlineLoading, LoadingState } from '../LoadingSpinner.jsx'
import { ResourceAccessPanel } from '../incident/ResourceAccessPanel.jsx'

function copyText(text) {
  if (!text) return Promise.resolve(false)
  if (navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(text).then(() => true).catch(() => false)
  }
  return Promise.resolve(false)
}

function YamlLine({ line, lineNo }) {
  const trimmed = line.trimStart()
  const indent = line.slice(0, line.length - trimmed.length)

  if (trimmed.startsWith('#')) {
    return (
      <div className="yaml-line">
        <span className="yaml-ln">{lineNo}</span>
        <span className="yaml-code yaml-comment">{line}</span>
      </div>
    )
  }

  const sep = trimmed.indexOf(':')
  if (sep > 0) {
    const key = trimmed.slice(0, sep)
    const rest = trimmed.slice(sep)
    const value = rest.slice(1).trimStart()
    const valueLead = rest.slice(1, rest.length - value.length)

    return (
      <div className="yaml-line">
        <span className="yaml-ln">{lineNo}</span>
        <span className="yaml-code">
          {indent}
          <span className="yaml-key">{key}</span>
          <span className="yaml-colon">:</span>
          {valueLead}
          <YamlValue value={value} />
        </span>
      </div>
    )
  }

  if (trimmed.startsWith('- ')) {
    const body = trimmed.slice(2)
    return (
      <div className="yaml-line">
        <span className="yaml-ln">{lineNo}</span>
        <span className="yaml-code">
          {indent}- <YamlValue value={body} inline />
        </span>
      </div>
    )
  }

  return (
    <div className="yaml-line">
      <span className="yaml-ln">{lineNo}</span>
      <span className="yaml-code">{line || ' '}</span>
    </div>
  )
}

function YamlValue({ value, inline = false }) {
  if (!value) return inline ? null : <span />
  if (value === '|' || value === '>') {
    return <span className="yaml-block-scalar">{value}</span>
  }
  if (value === 'null' || value === '~') {
    return <span className="yaml-null">{value}</span>
  }
  if (value === 'true' || value === 'false') {
    return <span className="yaml-bool">{value}</span>
  }
  if (/^-?\d+(\.\d+)?$/.test(value)) {
    return <span className="yaml-number">{value}</span>
  }
  if (
    (value.startsWith('"') && value.endsWith('"'))
    || (value.startsWith("'") && value.endsWith("'"))
  ) {
    return <span className="yaml-string">{value}</span>
  }
  return <span className="yaml-scalar">{value}</span>
}

const YamlHighlightedViewer = memo(function YamlHighlightedViewer({ lines }) {
  return (
    <pre className="yaml-viewer" aria-label="Resource manifest">
      {lines.map((line, i) => (
        <YamlLine key={i} line={line} lineNo={i + 1} />
      ))}
    </pre>
  )
})

function YamlPlainViewer({ text, lineCount = 0 }) {
  return (
    <>
      {lineCount > YAML_HIGHLIGHT_MAX_LINES && (
        <p className="resource-manifest-yaml-note muted">
          Large manifest ({lineCount.toLocaleString()} lines) — plain view for performance.
        </p>
      )}
      <pre className="yaml-viewer yaml-viewer-plain mono" aria-label="Resource manifest">{text}</pre>
    </>
  )
}

function ManifestYamlDisplay({ text }) {
  const meta = useMemo(() => getYamlRenderMeta(text), [text])
  const deferredText = useDeferredValue(text)
  const deferredMeta = useMemo(() => getYamlRenderMeta(deferredText), [deferredText])

  if (meta.mode === 'plain') {
    return <YamlPlainViewer text={text} lineCount={meta.lineCount} />
  }

  const highlightPending = deferredText !== text
  if (highlightPending) {
    return <YamlPlainViewer text={text} lineCount={meta.lineCount} />
  }

  return <YamlHighlightedViewer lines={deferredMeta.lines} />
}

export function ResourceManifestView({ target, cluster, onClose }) {
  const { manifest, loading, refreshing, error, refresh } = useResourceManifest(target, cluster)
  const command = manifest?.command || ''
  const yaml = manifest?.yaml || ''
  const renderMeta = useMemo(() => getYamlRenderMeta(yaml), [yaml])
  const deferredYaml = useDeferredValue(yaml)
  const highlighting = renderMeta.mode === 'highlight' && deferredYaml !== yaml
  const fetchError = error || manifest?.error || ''
  const accessDenied = isRbacForbiddenMessage(fetchError)

  const title = target
    ? `${target.kind}/${target.namespace ? `${target.namespace}/` : ''}${target.name}`
    : 'Manifest'

  const fetching = loading && !yaml

  async function handleCopyCommand() {
    await copyText(command)
  }

  async function handleCopyYaml() {
    await copyText(yaml)
  }

  return (
    <div className="resource-manifest">
      <div className="resource-manifest-toolbar">
        <div className="resource-manifest-title-block">
          <span className="resource-manifest-kicker">kubectl get -o yaml</span>
          <strong className="resource-manifest-title mono">{title}</strong>
        </div>
        <div className="resource-manifest-actions">
          {(refreshing || highlighting) && (
            <InlineLoading
              message={highlighting ? 'Rendering…' : 'Refreshing…'}
              className="resource-manifest-refreshing muted"
            />
          )}
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => refresh()}
            disabled={fetching}
            title="Refresh manifest"
          >
            Refresh
          </button>
          {onClose && (
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              onClick={onClose}
              title="Back to inspector"
            >
              Details
            </button>
          )}
        </div>
      </div>

      <div className="resource-manifest-command-wrap">
        <div className="resource-manifest-command-label">Command</div>
        <div className="resource-manifest-command-row">
          <code className="resource-manifest-command mono">{command || 'kubectl get … -o yaml'}</code>
          <button
            type="button"
            className="explorer-collapse-btn resource-manifest-copy"
            onClick={handleCopyCommand}
            disabled={!command}
            title="Copy kubectl command"
            aria-label="Copy kubectl command"
          >
            <CopyIcon />
          </button>
        </div>
      </div>

      <div className={['resource-manifest-body', fetching ? 'is-busy' : ''].filter(Boolean).join(' ')}>
        {fetching && (
          <LoadingState message="Fetching manifest…" />
        )}
        {fetchError && accessDenied && target && !fetching && (
          <ResourceAccessPanel
            action="get"
            kindGroup={{
              kind: target.kind,
              label: target.kind,
              resource: target.resource || String(target.kind || '').toLowerCase(),
              group: target.group || '',
              apiVersion: target.apiVersion || '',
              namespaced: !target.clusterScoped,
              accessState: 'forbidden',
            }}
          />
        )}
        {fetchError && !accessDenied && !fetching && (
          <div className="resource-manifest-error" role="alert">
            <strong>kubectl failed</strong>
            <pre className="mono">{fetchError}</pre>
          </div>
        )}
        {yaml && !fetchError && !fetching && (
          <>
            <div className="resource-manifest-yaml-head">
              <span className="muted">Manifest</span>
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                onClick={handleCopyYaml}
                title="Copy YAML"
              >
                Copy YAML
              </button>
            </div>
            <ManifestYamlDisplay text={yaml} />
          </>
        )}
        {!loading && !fetchError && !yaml && !fetching && (
          <p className="muted resource-manifest-empty">No manifest returned.</p>
        )}
      </div>
    </div>
  )
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5.5" y="5.5" width="7" height="7" rx="1.25" />
      <path d="M4.5 10.5h-1a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1h7a1 1 0 0 1 1 1v1" strokeLinecap="round" />
    </svg>
  )
}
