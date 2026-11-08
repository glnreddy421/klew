import { useEffect, useMemo, useState } from 'react'
import { SettingRow, SettingsGroup, SettingsMessage } from './SettingsControls.jsx'
import {
  AddExtensionFile,
  ExtensionsStatus,
  LinkExtensionFolder,
  OpenExtensionsFolder,
  ReloadExtensions,
  RemoveExtensionFile,
  TestExtensions,
  UnlinkExtensionFolder,
} from '../../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { entriesByFile, folderProblems, testSummary } from '../../lib/extensionsView.js'

const DOCS_URL = 'https://github.com/glnreddy421/klew/blob/main/docs/extensions.md'

/**
 * Settings → Extensions: add rule files, link folders of rules, see what
 * loaded (and why something was skipped), reload without a restart, and test
 * every rule against a namespace. The desktop app and the klew command share all of it.
 */
export function ExtensionsSettings({ cluster }) {
  const [view, setView] = useState(null)
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState(null)
  const [testNs, setTestNs] = useState(cluster?.selectedNamespace || '')
  const [test, setTest] = useState(null)

  useEffect(() => {
    ExtensionsStatus?.().then(setView).catch((err) => setMessage({ tone: 'error', text: String(err) }))
  }, [])
  useEffect(() => {
    if (!testNs && cluster?.selectedNamespace) setTestNs(cluster.selectedNamespace)
  }, [cluster?.selectedNamespace, testNs])

  async function act(label, fn, done) {
    setBusy(label)
    setMessage(null)
    try {
      const next = await fn()
      if (next) setView(next)
      if (done) setMessage({ tone: 'ok', text: done })
    } catch (err) {
      setMessage({ tone: 'error', text: String(err?.message || err) })
    } finally {
      setBusy('')
    }
  }

  async function runTest() {
    setBusy('test')
    setMessage(null)
    setTest(null)
    try {
      setTest(await TestExtensions(testNs))
    } catch (err) {
      setMessage({ tone: 'error', text: String(err?.message || err) })
    } finally {
      setBusy('')
    }
  }

  const files = useMemo(() => entriesByFile(view?.entries || []), [view])
  const problems = useMemo(() => folderProblems(view?.problems || []), [view])
  const ownFolder = view?.ownFolder || ''
  const namespaces = cluster?.namespaces || []

  return (
    <>
      <SettingsGroup
        title="Sources"
        description="Add a single file, which Klew copies into its own folder, or link a folder of your own (a git repository of rules, say), which Klew reads in place. Edits apply when you Reload."
        footer={message ? <SettingsMessage>{message.text}</SettingsMessage> : null}
      >
        <SettingRow label="Klew's folder" hint={ownFolder || 'Not available'}>
          <div className="settings-field-row">
            <button type="button" className="btn btn-primary btn-sm" disabled={Boolean(busy)} onClick={() => act('add', AddExtensionFile)}>
              Add file…
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => OpenExtensionsFolder(ownFolder)}>Open</button>
          </div>
        </SettingRow>
        {(view?.linked || []).map((f) => (
          <SettingRow
            key={f.path}
            label={<span>Linked folder{!f.found && <span className="ext-warn"> · not found</span>}</span>}
            hint={f.path}
          >
            <div className="settings-field-row">
              <button type="button" className="btn btn-ghost btn-sm" disabled={!f.found} onClick={() => OpenExtensionsFolder(f.path)}>Open</button>
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                disabled={Boolean(busy)}
                onClick={() => act('unlink', () => UnlinkExtensionFolder(f.path), 'Unlinked. Its files were left untouched.')}
              >
                Unlink
              </button>
            </div>
          </SettingRow>
        ))}
        <SettingRow label="Link a folder" hint="Every .yaml and .yml file in it is read, by the app and the klew command alike.">
          <div className="settings-field-row">
            <button type="button" className="btn btn-ghost btn-sm" disabled={Boolean(busy)} onClick={() => act('link', LinkExtensionFolder)}>
              Link folder…
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-sm"
              disabled={Boolean(busy)}
              onClick={() => act('reload', ReloadExtensions, 'Reloaded. New investigations, and the next refresh of a running one, use these.')}
            >
              {busy === 'reload' ? 'Reloading…' : 'Reload'}
            </button>
          </div>
        </SettingRow>
      </SettingsGroup>

      <SettingsGroup
        title="Loaded"
        description={view ? `${view.collectors} collector${view.collectors === 1 ? '' : 's'} and ${view.rules} rule${view.rules === 1 ? '' : 's'} in use.` : 'Loading…'}
      >
        {view && files.length === 0 && (
          <SettingRow
            label="No extensions yet"
            hint="Add a file or link a folder. Built-in collectors and rules always run."
          >
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => BrowserOpenURL(DOCS_URL)}>How to write one ↗</button>
          </SettingRow>
        )}
        {files.map((file) => (
          <div key={file.path} className="settings-row is-stacked ext-file">
            <div className="ext-file-head">
              <span className="mono ext-file-name" title={file.path}>{file.name}</span>
              <span className="muted ext-file-folder" title={file.folder}>{file.folder === ownFolder ? "Klew's folder" : file.folder}</span>
              {file.folder === ownFolder && (
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  disabled={Boolean(busy)}
                  onClick={() => {
                    if (window.confirm(`Remove ${file.name} from Klew's folder?`)) act('remove', () => RemoveExtensionFile(file.path))
                  }}
                >
                  Remove
                </button>
              )}
            </div>
            <ul className="ext-docs">
              {file.entries.map((e) => (
                <li key={`${e.document}`} className={`ext-doc is-${e.status}`}>
                  <span className="ext-dot" aria-hidden="true" />
                  <span className="ext-kind">{e.kind || 'Document'}</span>
                  <span className="mono">{e.name || `document ${e.document}`}</span>
                  <span className="muted">{e.status === 'loaded' ? 'loaded' : 'skipped'}</span>
                  {e.reason && <span className="ext-reason">{e.reason}</span>}
                </li>
              ))}
            </ul>
          </div>
        ))}
        {problems.map((p) => (
          <SettingRow key={p} label={<span className="ext-warn">Problem</span>} hint={p} />
        ))}
      </SettingsGroup>

      <SettingsGroup
        title="Test"
        description="Run every rule against a namespace and see what each one matches. Klew reads the namespace once, without logs, and changes nothing."
      >
        <SettingRow label="Namespace">
          <div className="settings-field-row">
            <select
              className="settings-input settings-select-compact ext-ns-select"
              value={testNs}
              onChange={(e) => setTestNs(e.target.value)}
              aria-label="Namespace to test against"
            >
              {!namespaces.includes(testNs) && testNs && <option value={testNs}>{testNs}</option>}
              {namespaces.map((ns) => <option key={ns} value={ns}>{ns}</option>)}
            </select>
            <button type="button" className="btn btn-primary btn-sm" disabled={!testNs || !view?.rules || Boolean(busy)} onClick={runTest}>
              {busy === 'test' ? 'Testing…' : 'Test rules'}
            </button>
          </div>
        </SettingRow>
        {view && !view.rules && <SettingRow hint="Add or link a rule to test it." />}
        {test && testSummary(test).map((run) => (
          <div key={run.rule} className="settings-row is-stacked ext-run">
            <div className="ext-file-head">
              <span className="mono ext-file-name">{run.rule}</span>
              <span className="muted">{run.resource}</span>
              <span className={`ext-count ${run.errors ? 'has-errors' : ''}`}>
                {run.matched} of {run.checked} matched{run.errors ? ` · ${run.errors} errors` : ''}
              </span>
            </div>
            {run.checked === 0 && <p className="muted ext-reason">No {run.resource} objects here, so the rule cannot match anything.</p>}
            <ul className="ext-docs">
              {run.lines.map((line) => (
                <li key={line.key} className={`ext-doc is-${line.kind}`}>
                  <span className="ext-dot" aria-hidden="true" />
                  <span className="mono">{line.object}</span>
                  {line.text && <span className="ext-reason">{line.text}</span>}
                </li>
              ))}
              {run.hidden > 0 && <li className="muted ext-more">and {run.hidden} more</li>}
            </ul>
            {run.warnings.map((w) => <p key={w} className="ext-reason ext-warn">{w}</p>)}
          </div>
        ))}
        {test?.warnings?.length > 0 && (
          <SettingRow
            label={<span className="ext-warn">Not fully read</span>}
            hint={test.warnings.slice(0, 3).join(' · ')}
          />
        )}
      </SettingsGroup>
    </>
  )
}
