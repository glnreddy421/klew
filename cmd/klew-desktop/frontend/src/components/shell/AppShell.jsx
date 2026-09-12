import { useCallback, useMemo } from 'react'
import { ActivityRail } from './ActivityRail.jsx'
import { ContextExplorer } from './ContextExplorer.jsx'
import { InspectorPanel } from './InspectorPanel.jsx'
import { TopBar } from '../TopBar.jsx'
import { useShellLayout } from '../../hooks/useShellLayout.js'
import { ShellInspectorProvider } from '../../context/ShellInspectorContext.jsx'
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
  terminalOpen = false,
  liveLogsOpen = false,
  streamLive = null,
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
  onOpenPodLogs,
  explorerFilters,
  onExplorerFiltersChange,
  graphRelations,
  onGraphRelationsChange,
  investigationActive = false,
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
  const inspectorLayoutCollapsed = showInspector && layout.inspectorCollapsed
  const inspectorBottom = layout.inspectorPlacement === 'bottom'

  const expandInspector = useCallback(() => {
    if (!showInspector) return
    patch({ inspectorCollapsed: false })
    if (!inspectorBottom && layout.inspectorWidth < INSPECTOR_AUTO_WIDTH) {
      setInspectorWidth(INSPECTOR_AUTO_WIDTH)
    }
  }, [showInspector, inspectorBottom, layout.inspectorWidth, patch, setInspectorWidth])

  const shellInspector = useMemo(
    () => ({ expandInspector, onOpenPodLogs }),
    [expandInspector, onOpenPodLogs],
  )

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

  const inspectorPanel = showInspector && !inspectorLayoutCollapsed && (
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
        aria-label="Inspector"
      >
        <InspectorPanel
          placement={inspectorBottom ? 'bottom' : 'right'}
          onPlacementChange={setInspectorPlacement}
          onClose={toggleInspector}
          onOpenPodLogs={onOpenPodLogs}
          cluster={cluster}
        >
          {inspector}
        </InspectorPanel>
      </aside>
    </>
  )

  return (
    <ShellInspectorProvider value={shellInspector}>
    <div className="app-shell">
      <TopBar {...topBarProps} onOpenSettings={onOpenSettings} onOpenHelp={onOpenHelp} />

      <div className="app-shell-body">
        <ActivityRail
          active={tab}
          terminalOpen={terminalOpen}
          liveLogsOpen={liveLogsOpen}
          streamLive={streamLive}
          onSelect={onTabChange}
          collapsed={layout.railCollapsed}
          onToggleCollapse={toggleRail}
        />

        <div className="app-shell-main">
          <div className={`app-shell-panes ${inspectorBottom ? 'inspector-bottom' : 'inspector-right'}`}>
            {showExplorer && tab !== 'settings' && tab !== 'incident' && tab !== 'nodes' && (
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
                  investigationActive={investigationActive}
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
          </div>

          {inspectorBottom && inspectorPanel}
        </div>
      </div>
    </div>
    </ShellInspectorProvider>
  )
}
