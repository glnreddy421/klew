import { useCallback, useEffect, useRef } from 'react'
import { ActivityRail } from './ActivityRail.jsx'
import { ContextExplorer } from './ContextExplorer.jsx'
import { TopBar } from '../TopBar.jsx'
import { useShellLayout } from '../../hooks/useShellLayout.js'
import { defaultRelations } from './explorers/ExplorerPanels.jsx'

const EXPLORER_MIN = 180
const EXPLORER_MAX = 320
const INSPECTOR_MIN = 300
const INSPECTOR_MAX = 720
const INSPECTOR_BOTTOM_MIN = 200
const INSPECTOR_BOTTOM_MAX = 560
const INSPECTOR_AUTO_WIDTH = 440

export function AppShell({
  tab,
  onTabChange,
  onOpenSettings,
  onOpenHelp,
  topBarProps,
  showExplorer = true,
  showInspector = false,
  inspector,
  children,
  view,
  cluster,
  activeQuery,
  timeWindowLabel,
  live,
  prefs,
  onPrefsChange,
  inspectRow,
  explorerFilters,
  onExplorerFiltersChange,
  graphRelations,
  onGraphRelationsChange,
}) {
  const {
    layout,
    toggleExplorer,
    toggleInspector,
    toggleRail,
    setExplorerWidth,
    setInspectorWidth,
    setInspectorPlacement,
    patch,
  } = useShellLayout(tab)

  const explorerCollapsed = layout.explorerCollapsed
  const inspectorCollapsed = layout.inspectorCollapsed || !showInspector
  const inspectorBottom = layout.inspectorPlacement === 'bottom'

  const lastAutoExpandKeyRef = useRef(null)

  useEffect(() => {
    if (!showInspector || !inspectRow?.key) return
    if (lastAutoExpandKeyRef.current === inspectRow.key) return
    lastAutoExpandKeyRef.current = inspectRow.key
    if (layout.inspectorCollapsed) {
      patch({ inspectorCollapsed: false })
    }
    if (!inspectorBottom && layout.inspectorWidth < INSPECTOR_AUTO_WIDTH) {
      setInspectorWidth(INSPECTOR_AUTO_WIDTH)
    }
  }, [inspectRow?.key, showInspector, inspectorBottom, layout.inspectorCollapsed, layout.inspectorWidth, patch, setInspectorWidth])

  const startExplorerResize = useCallback((e) => {
    e.preventDefault()
    const startX = e.clientX
    const startW = layout.explorerWidth
    function onMove(ev) {
      const next = Math.min(EXPLORER_MAX, Math.max(EXPLORER_MIN, startW + (ev.clientX - startX)))
      setExplorerWidth(next)
    }
    function onUp() {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }, [layout.explorerWidth, setExplorerWidth])

  const startInspectorResize = useCallback((e) => {
    e.preventDefault()
    if (inspectorBottom) {
      const startY = e.clientY
      const startH = layout.inspectorWidth
      function onMove(ev) {
        const next = Math.min(INSPECTOR_BOTTOM_MAX, Math.max(INSPECTOR_BOTTOM_MIN, startH - (ev.clientY - startY)))
        setInspectorWidth(next)
      }
      function onUp() {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
      }
      document.body.style.cursor = 'row-resize'
      document.body.style.userSelect = 'none'
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
      return
    }

    const startX = e.clientX
    const startW = layout.inspectorWidth
    function onMove(ev) {
      const next = Math.min(INSPECTOR_MAX, Math.max(INSPECTOR_MIN, startW - (ev.clientX - startX)))
      setInspectorWidth(next)
    }
    function onUp() {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }, [inspectorBottom, layout.inspectorWidth, setInspectorWidth])

  const relations = graphRelations || defaultRelations()

  const inspectorStyle = inspectorBottom
    ? {
      height: `${layout.inspectorWidth}px`,
      minHeight: `${INSPECTOR_BOTTOM_MIN}px`,
      maxHeight: '42vh',
    }
    : { width: `${layout.inspectorWidth}px`, minWidth: `${layout.inspectorWidth}px` }

  const inspectorPanel = showInspector && !inspectorCollapsed && (
    <>
      <div
        className={`pane-resize-handle ${inspectorBottom ? 'pane-resize-handle-row' : ''}`}
        role="separator"
        aria-orientation={inspectorBottom ? 'horizontal' : 'vertical'}
        aria-label="Resize inspector"
        onMouseDown={startInspectorResize}
      />
      <aside
        className={`app-inspector ${inspectorBottom ? 'app-inspector-bottom' : ''}`}
        style={inspectorStyle}
      >
        <header className="inspector-header">
          <h2 className="inspector-header-title">Inspector</h2>
          <div className="inspector-header-actions">
            <div className="inspector-placement-toggle" role="group" aria-label="Inspector placement">
              <button
                type="button"
                className={`inspector-placement-btn ${!inspectorBottom ? 'active' : ''}`}
                onClick={() => setInspectorPlacement('right')}
                title="Dock inspector on the right"
                aria-label="Dock inspector on the right"
                aria-pressed={!inspectorBottom}
              >
                Right
              </button>
              <button
                type="button"
                className={`inspector-placement-btn ${inspectorBottom ? 'active' : ''}`}
                onClick={() => setInspectorPlacement('bottom')}
                title="Dock inspector on the bottom"
                aria-label="Dock inspector on the bottom"
                aria-pressed={inspectorBottom}
              >
                Bottom
              </button>
            </div>
            <button
              type="button"
              className="explorer-collapse-btn"
              onClick={toggleInspector}
              title="Collapse inspector"
              aria-label="Collapse inspector"
            >
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
                <path d={inspectorBottom ? 'M4 10l4-4 4 4' : 'M6 4l4 4-4 4'} strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          </div>
        </header>
        <div className="inspector-body">
          {inspector}
        </div>
      </aside>
    </>
  )

  return (
    <div className="app-shell">
      <TopBar {...topBarProps} onOpenSettings={onOpenSettings} onOpenHelp={onOpenHelp} />

      <div className="app-shell-body">
        <ActivityRail
          active={tab}
          onSelect={onTabChange}
          collapsed={layout.railCollapsed}
          onToggleCollapse={toggleRail}
        />

        <div className="app-shell-main">
          <div className={`app-shell-panes ${inspectorBottom ? 'inspector-bottom' : 'inspector-right'}`}>
            {showExplorer && tab !== 'settings' && tab !== 'incident' && tab !== 'nodes' && tab !== 'terminal' && (
              <>
                <ContextExplorer
                  tab={tab}
                  collapsed={explorerCollapsed}
                  onToggleCollapse={toggleExplorer}
                  view={view}
                  cluster={cluster}
                  activeQuery={activeQuery}
                  timeWindowLabel={timeWindowLabel}
                  live={live}
                  prefs={prefs}
                  onPrefsChange={onPrefsChange}
                  explorerFilters={explorerFilters || {}}
                  onExplorerFiltersChange={onExplorerFiltersChange}
                  graphRelations={relations}
                  onGraphRelationsChange={onGraphRelationsChange}
                  inspectRow={inspectRow}
                  width={layout.explorerWidth}
                />
                {!explorerCollapsed && (
                  <div
                    className="pane-resize-handle"
                    role="separator"
                    aria-orientation="vertical"
                    aria-label="Resize explorer"
                    onMouseDown={startExplorerResize}
                  />
                )}
              </>
            )}

            <main className="app-workspace">
              {children}
            </main>

            {!inspectorBottom && inspectorPanel}

            {showInspector && inspectorCollapsed && (
              <div className="app-inspector-collapsed">
                <button
                  type="button"
                  className="explorer-expand-btn"
                  onClick={toggleInspector}
                  title="Expand inspector"
                  aria-label="Expand inspector"
                >
                  <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
                    <path d="M10 4L6 8l4 4" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                </button>
              </div>
            )}
          </div>

          {inspectorBottom && inspectorPanel}
        </div>
      </div>
    </div>
  )
}
