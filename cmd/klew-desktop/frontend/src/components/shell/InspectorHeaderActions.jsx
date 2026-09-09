import { useEffect, useRef, useState } from 'react'

const POD_LOGS_TITLE = 'Tail live pod logs in a new terminal tab'
const POD_LOGS_DISABLED = 'Select a pod to tail logs in terminal'

export function InspectorHeaderActions({
  placement = 'right',
  onPlacementChange,
  onClose,
  manifestOpen = false,
  manifestEnabled = false,
  onToggleManifest,
  podLogsEnabled = false,
  podLogsContainers = [],
  onOpenPodLogs,
}) {
  const [open, setOpen] = useState(false)
  const [containerMenuOpen, setContainerMenuOpen] = useState(false)
  const anchorRef = useRef(null)
  const panelRef = useRef(null)
  const containerAnchorRef = useRef(null)
  const containerPanelRef = useRef(null)

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
      if (containerAnchorRef.current?.contains(e.target)) return
      if (containerPanelRef.current?.contains(e.target)) return
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
  }, [podLogsEnabled, podLogsContainers])

  function launchPodLogs(container = '') {
    onOpenPodLogs?.({ container })
    setContainerMenuOpen(false)
  }

  function handlePodLogsClick() {
    if (!podLogsEnabled) return
    if (podLogsContainers.length > 1) {
      setContainerMenuOpen((v) => !v)
      return
    }
    launchPodLogs(podLogsContainers[0]?.name || '')
  }

  return (
    <div className="inspector-header-actions">
      <div className="inspector-split-anchor" ref={containerAnchorRef}>
        <button
          type="button"
          className={[
            'explorer-collapse-btn',
            'inspector-pod-logs-btn',
            containerMenuOpen ? 'is-open' : '',
            !podLogsEnabled ? 'is-disabled' : '',
          ].filter(Boolean).join(' ')}
          onClick={handlePodLogsClick}
          disabled={!podLogsEnabled}
          title={podLogsEnabled ? POD_LOGS_TITLE : POD_LOGS_DISABLED}
          aria-label="Tail live pod logs in terminal"
          aria-expanded={containerMenuOpen}
          aria-haspopup={podLogsContainers.length > 1 ? 'menu' : undefined}
        >
          <PodLogsTerminalIcon />
        </button>
        {containerMenuOpen && podLogsContainers.length > 1 && (
          <div
            ref={containerPanelRef}
            className="inspector-split-menu inspector-container-menu"
            role="menu"
            aria-label="Select container"
          >
            <div className="inspector-container-menu-label">Container</div>
            {podLogsContainers.map((container) => (
              <button
                key={`${container.init ? 'init' : 'app'}-${container.name}`}
                type="button"
                role="menuitem"
                className="inspector-split-option"
                onClick={() => launchPodLogs(container.name)}
              >
                <ContainerGlyph init={container.init} />
                <span className="mono">{container.name}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <button
        type="button"
        className={[
          'explorer-collapse-btn',
          'inspector-manifest-btn',
          manifestOpen ? 'is-active' : '',
          !manifestEnabled ? 'is-disabled' : '',
        ].filter(Boolean).join(' ')}
        onClick={onToggleManifest}
        disabled={!manifestEnabled}
        title={manifestEnabled ? 'View kubectl manifest (YAML)' : 'Select a resource to view manifest'}
        aria-label="View kubectl manifest"
        aria-pressed={manifestOpen}
      >
        <KubectlIcon />
      </button>
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

/** Folded YAML document — distinct from the terminal window icon. */
function KubectlIcon() {
  return (
    <InspectorIcon>
      <path d="M4.25 2.75h5.35L12.25 5.4V12.8a1 1 0 01-1 1H4.25a1 1 0 01-1-1V3.75a1 1 0 011-1z" />
      <path d="M9.6 2.75V5.4H12.25" />
      <path d="M5.35 7.15h5.3" />
      <path d="M5.35 9.05h3.35" />
      <path d="M5.35 10.95h4.55" />
    </InspectorIcon>
  )
}

/** Terminal shell with log lines + live tail dot. */
function PodLogsTerminalIcon() {
  return (
    <InspectorIcon>
      <rect x="2.25" y="2.75" width="11.5" height="10.5" rx="1.35" />
      <path d="M5 5.35h4.75" />
      <path d="M5 7.05h3.35" opacity="0.82" />
      <path d="M5 8.75h4.1" opacity="0.62" />
      <path d="M4.55 11.35L5.95 12.55L4.55 13.75" />
      <path d="M7.15 12.55h3.35" />
      <circle cx="12.35" cy="4.15" r="1.15" fill="currentColor" stroke="none" />
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
