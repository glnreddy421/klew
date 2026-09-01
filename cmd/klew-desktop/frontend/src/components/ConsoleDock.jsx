import { DOCK_VIEW_SPLIT, DOCK_VIEW_STREAM, DOCK_VIEW_TERMINAL } from '../hooks/useConsoleDock'
import { PANEL_NORMAL } from '../hooks/useStreamPanel'
import { LiveStreamPanel } from './LiveStreamPanel'
import { TerminalPanel } from './TerminalPanel'
import { StreamLiveBadge } from './StreamLiveBadge'

export function ConsoleDock({
  visible = true,
  activeView,
  expanded,
  height,
  maximized,
  onSelectView,
  onMinimize,
  onMaximize,
  onRestore,
  onClose,
  onResizeStart,
  streamLive,
  streamProps,
  terminalProps,
}) {
  if (!visible) return null

  const showTerminal = expanded && (activeView === DOCK_VIEW_TERMINAL || activeView === DOCK_VIEW_SPLIT)
  const showStream = expanded && (activeView === DOCK_VIEW_STREAM || activeView === DOCK_VIEW_SPLIT)
  const split = activeView === DOCK_VIEW_SPLIT

  return (
    <div
      className={[
        'console-dock',
        expanded ? 'is-expanded' : 'is-collapsed',
        maximized ? 'is-maximized' : '',
        split ? 'is-split' : '',
      ].filter(Boolean).join(' ')}
      aria-label="Console dock"
    >
      {expanded && !maximized && (
        <div
          className="stream-resize-handle"
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize console"
          onMouseDown={onResizeStart}
        />
      )}

      <div className="console-dock-tabs" role="tablist" aria-label="Console panels">
        <DockTab
          id={DOCK_VIEW_TERMINAL}
          label="Terminal"
          active={activeView === DOCK_VIEW_TERMINAL || (split && activeView === DOCK_VIEW_SPLIT)}
          pressed={activeView === DOCK_VIEW_TERMINAL}
          onSelect={() => onSelectView(DOCK_VIEW_TERMINAL)}
        />
        <DockTab
          id={DOCK_VIEW_STREAM}
          label="Live logs"
          active={activeView === DOCK_VIEW_STREAM || split}
          pressed={activeView === DOCK_VIEW_STREAM}
          onSelect={() => onSelectView(DOCK_VIEW_STREAM)}
          badge={<StreamLiveBadge {...streamLive} />}
        />
        <button
          type="button"
          className={`console-dock-tab console-dock-tab-split ${split ? 'is-active' : ''}`}
          role="tab"
          aria-selected={split}
          title="Show terminal and live tail side by side"
          onClick={() => onSelectView(DOCK_VIEW_SPLIT)}
        >
          Split
        </button>

        <div className="console-dock-spacer" />

        {expanded && (
          <div className="console-dock-window-controls">
            {maximized ? (
              <WindowBtn label="Restore" onClick={onRestore}>⛶</WindowBtn>
            ) : (
              <WindowBtn label="Maximize" onClick={onMaximize}>⛶</WindowBtn>
            )}
            <WindowBtn label="Collapse" onClick={onMinimize}>▼</WindowBtn>
            <WindowBtn label="Close console" onClick={onClose}>×</WindowBtn>
          </div>
        )}
      </div>

      {expanded && (
        <div
          className="console-dock-body"
          style={maximized ? undefined : { height }}
        >
          {showTerminal && (
            <div className={`console-dock-pane console-dock-pane-terminal ${split ? 'is-split-pane' : ''}`}>
              <TerminalPanel
                {...terminalProps}
                embedded
                open
                height={split ? undefined : height}
                panelState={PANEL_NORMAL}
              />
            </div>
          )}
          {showStream && (
            <div className={`console-dock-pane console-dock-pane-stream ${split ? 'is-split-pane' : ''}`}>
              <LiveStreamPanel
                {...streamProps}
                embedded
                panelState={PANEL_NORMAL}
                height={split ? undefined : height}
              />
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function DockTab({ id, label, active, pressed, onSelect, badge = null }) {
  return (
    <button
      type="button"
      id={`console-dock-tab-${id}`}
      className={`console-dock-tab ${active ? 'is-active' : ''}`}
      role="tab"
      aria-selected={pressed}
      aria-controls={`console-dock-panel-${id}`}
      title={label}
      onClick={onSelect}
    >
      <span className="console-dock-tab-label">{label}</span>
      {badge}
    </button>
  )
}

function WindowBtn({ label, onClick, children }) {
  return (
    <button type="button" className="stream-icon-btn" aria-label={label} onClick={onClick}>
      {children}
    </button>
  )
}
