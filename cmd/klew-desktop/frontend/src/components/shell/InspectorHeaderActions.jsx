import { useEffect, useRef, useState } from 'react'
import { InvestigateIcon } from '../InvestigateIcon.jsx'
import { StreamLogsGlyph } from '../DockLauncher.jsx'

export function InspectorHeaderActions({
  placement = 'right',
  onPlacementChange,
  onClose,
  podStreamContainers = [],
  investigateEnabled = false,
  investigateTitle = '',
  onInvestigate,
  streamLogsEnabled = false,
  streamLogsTitle = '',
  onStreamLogs,
}) {
  const [open, setOpen] = useState(false)
  const [containerMenuOpen, setContainerMenuOpen] = useState(false)
  const anchorRef = useRef(null)
  const panelRef = useRef(null)
  const streamAnchorRef = useRef(null)
  const streamPanelRef = useRef(null)

  useEffect(() => {
    if (!open) return undefined
    function onDoc(e) {
      if (anchorRef.current?.contains(e.target)) return
      if (panelRef.current?.contains(e.target)) return
      setOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  useEffect(() => {
    if (!containerMenuOpen) return undefined
    function onDoc(e) {
      if (streamAnchorRef.current?.contains(e.target)) return
      if (streamPanelRef.current?.contains(e.target)) return
      setContainerMenuOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setContainerMenuOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [containerMenuOpen])

  useEffect(() => {
    setContainerMenuOpen(false)
  }, [streamLogsEnabled, podStreamContainers])

  function launchStream(container = '') {
    onStreamLogs?.(container)
    setContainerMenuOpen(false)
  }

  function handleStreamClick() {
    if (!streamLogsEnabled || !onStreamLogs) return
    if (podStreamContainers.length > 1) {
      setContainerMenuOpen((v) => !v)
      return
    }
    launchStream('')
  }

  return (
    <div className="inspector-header-actions">
      {onInvestigate && (
        <button
          type="button"
          className="inspector-investigate-btn"
          onClick={onInvestigate}
          disabled={!investigateEnabled}
          title={investigateTitle}
          aria-label={investigateTitle}
        >
          <InvestigateIcon size={14} />
        </button>
      )}
      {onStreamLogs && (
        <div className="inspector-split-anchor" ref={streamAnchorRef}>
          <button
            type="button"
            className={[
              'explorer-collapse-btn',
              'inspector-stream-logs-btn',
              containerMenuOpen ? 'is-open' : '',
              !streamLogsEnabled ? 'is-disabled' : '',
            ].filter(Boolean).join(' ')}
            onClick={handleStreamClick}
            disabled={!streamLogsEnabled}
            title={streamLogsEnabled ? streamLogsTitle : 'Select a pod, workload or Service to stream its logs'}
            aria-label="Stream pod logs in Live logs"
            aria-expanded={podStreamContainers.length > 1 ? containerMenuOpen : undefined}
            aria-haspopup={podStreamContainers.length > 1 ? 'menu' : undefined}
          >
            <StreamLogsGlyph size={16} />
          </button>
          {containerMenuOpen && podStreamContainers.length > 1 && (
            <div
              ref={streamPanelRef}
              className="inspector-split-menu inspector-container-menu"
              role="menu"
              aria-label="Stream logs"
            >
              <div className="inspector-container-menu-label">Container</div>
              <button type="button" role="menuitem" className="inspector-split-option" onClick={() => launchStream('')}>
                <span>All containers</span>
              </button>
              {podStreamContainers.map((container) => (
                <button
                  key={`${container.init ? 'init' : 'app'}-${container.name}`}
                  type="button"
                  role="menuitem"
                  className="inspector-split-option"
                  onClick={() => launchStream(container.name)}
                >
                  <ContainerGlyph init={container.init} />
                  <span className="mono">{container.name}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
      <div className="inspector-split-anchor" ref={anchorRef}>
        <button
          type="button"
          className={`explorer-collapse-btn inspector-split-btn ${open ? 'is-open' : ''}`}
          onClick={() => setOpen((v) => !v)}
          title="Inspector layout"
          aria-label="Inspector layout"
          aria-expanded={open}
          aria-haspopup="menu"
        >
          <SplitLayoutIcon active={placement} />
        </button>
        {open && (
          <div ref={panelRef} className="inspector-split-menu" role="menu" aria-label="Inspector layout">
            <button
              type="button"
              role="menuitemradio"
              className={`inspector-split-option ${placement === 'right' ? 'is-active' : ''}`}
              aria-checked={placement === 'right'}
              onClick={() => {
                onPlacementChange?.('right')
                setOpen(false)
              }}
            >
              <SplitRightIcon />
              <span>Split right</span>
            </button>
            <button
              type="button"
              role="menuitemradio"
              className={`inspector-split-option ${placement === 'bottom' ? 'is-active' : ''}`}
              aria-checked={placement === 'bottom'}
              onClick={() => {
                onPlacementChange?.('bottom')
                setOpen(false)
              }}
            >
              <SplitBottomIcon />
              <span>Split bottom</span>
            </button>
          </div>
        )}
      </div>
      <button
        type="button"
        className="explorer-collapse-btn inspector-close-btn"
        onClick={onClose}
        title="Close inspector"
        aria-label="Close inspector"
      >
        <CloseIcon />
      </button>
    </div>
  )
}

function SplitLayoutIcon({ active = 'right' }) {
  return active === 'bottom' ? <SplitBottomIcon /> : <SplitRightIcon />
}

function InspectorIcon(props) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.55"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...props}
    />
  )
}

function SplitRightIcon() {
  return (
    <InspectorIcon>
      <rect x="2.25" y="2.75" width="11.5" height="10.5" rx="1.35" />
      <path d="M10.25 2.75v10.5" />
      <rect x="10.5" y="3.1" width="3" height="10" rx="0.6" fill="currentColor" stroke="none" opacity="0.24" />
    </InspectorIcon>
  )
}

function SplitBottomIcon() {
  return (
    <InspectorIcon>
      <rect x="2.25" y="2.75" width="11.5" height="10.5" rx="1.35" />
      <path d="M2.25 9.25h11.5" />
      <rect x="2.6" y="9.5" width="10.8" height="3.5" rx="0.6" fill="currentColor" stroke="none" opacity="0.24" />
    </InspectorIcon>
  )
}

function CloseIcon() {
  return (
    <InspectorIcon strokeWidth="1.85">
      <path d="M4.25 4.25l7.5 7.5M11.75 4.25l-7.5 7.5" />
    </InspectorIcon>
  )
}

function ContainerGlyph({ init = false }) {
  return (
    <InspectorIcon width="14" height="14" strokeWidth="1.45">
      <path d="M3.75 5.25 8 3.25l4.25 2v5.5L8 13.25l-4.25-2z" />
      <path d="M8 8.25v5" />
      <path d="M12.25 5.25 8 8.25 3.75 5.25" />
      {init ? (
        <>
          <path d="M6.15 10.35h3.7" />
          <path d="M6.15 11.55h2.2" opacity="0.75" />
        </>
      ) : (
        <path d="M6.15 10.95h3.7" />
      )}
    </InspectorIcon>
  )
}
