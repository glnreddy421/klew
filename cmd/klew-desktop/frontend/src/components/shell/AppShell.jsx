import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  clampSize,
  maxExplorerWidth,
  maxInspectorHeight,
  maxInspectorWidth,
  openInspectorPaneSize,
  runPointerColResize,
  runPointerDragResize,
  runPointerRowResize,
} from '../../lib/paneDragResize.js'
import { shouldDismissInspector } from '../../lib/inspectorDismiss.js'
import {
  RAIL_WIDTH,
  WORKSPACE_MIN_WIDTH,
  nextRailMode,
  resolveRailCollapsed,
  shouldAutoCollapseExplorer,
} from '../../lib/shellLayout.js'
import { ActivityRail } from './ActivityRail.jsx'
import { ContextExplorer } from './ContextExplorer.jsx'
import { InspectorPanel } from './InspectorPanel.jsx'
import { TopBar } from '../TopBar.jsx'
import { useShellLayout } from '../../hooks/useShellLayout.js'
import { ShellInspectorProvider } from '../../context/ShellInspectorContext.jsx'
import { defaultRelations } from './explorers/ExplorerPanels.jsx'
const EXPLORER_MIN = 180
const INSPECTOR_MIN = 280
const INSPECTOR_BOTTOM_MIN = 200
const SHUTTER_DRAG_THRESHOLD = 5
/** Extra room required before an auto-collapsed explorer comes back (avoids flicker while dragging). */
const EXPLORER_RESTORE_HYSTERESIS = 48
const TABS_WITHOUT_EXPLORER = new Set(['settings', 'incident', 'nodes'])

function useViewportWidth() {
  const [width, setWidth] = useState(() => window.innerWidth)
  useEffect(() => {
    const onResize = () => setWidth(window.innerWidth)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])
  return width
}

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
    setRailMode,
    setExplorerWidth,
    setInspectorWidth,
    setInspectorPlacement,
    patch,
  } = useShellLayout(tab)

  // Latest layout for callbacks that must stay referentially stable (the shell
  // context would otherwise change on every drag frame).
  const layoutRef = useRef(layout)
  layoutRef.current = layout
  const explorerAutoWidthRef = useRef(null)

  const viewportWidth = useViewportWidth()
  const hasExplorer = showExplorer && !TABS_WITHOUT_EXPLORER.has(tab)
  const railCollapsed = resolveRailCollapsed(layout.railMode, hasExplorer)
  const toggleRail = useCallback(() => {
    setRailMode(nextRailMode(layout.railMode, hasExplorer))
  }, [hasExplorer, layout.railMode, setRailMode])

  const inspectorLayoutCollapsed = showInspector && layout.inspectorCollapsed
  const inspectorBottom = layout.inspectorPlacement === 'bottom'

  // Explorer yields to an open right-side inspector when the workspace would get
  // too narrow, and returns when the inspector closes. Expanding it by hand wins
  // until the inspector closes.
  const inspectorOpenRight = showInspector && !layout.inspectorCollapsed && !inspectorBottom
  const [explorerAutoSuppressed, setExplorerAutoSuppressed] = useState(false)
  const explorerAutoCollapsedRef = useRef(false)
  useEffect(() => {
    if (!inspectorOpenRight) setExplorerAutoSuppressed(false)
  }, [inspectorOpenRight])
  const explorerAutoCollapsed = hasExplorer
    && inspectorOpenRight
    && !layout.explorerCollapsed
    && !explorerAutoSuppressed
    && shouldAutoCollapseExplorer({
      viewportWidth,
      railWidth: railCollapsed ? RAIL_WIDTH.collapsed : RAIL_WIDTH.expanded,
      explorerWidth: layout.explorerWidth,
      inspectorWidth: layout.inspectorWidth,
      minWorkspace: WORKSPACE_MIN_WIDTH
        + (explorerAutoCollapsedRef.current ? EXPLORER_RESTORE_HYSTERESIS : 0),
    })
  explorerAutoCollapsedRef.current = explorerAutoCollapsed
  const explorerCollapsed = layout.explorerCollapsed || explorerAutoCollapsed

  const handleToggleExplorer = useCallback(() => {
    if (explorerAutoCollapsed) {
      setExplorerAutoSuppressed(true)
      return
    }
    toggleExplorer()
  }, [explorerAutoCollapsed, toggleExplorer])
  const inspectorMin = inspectorBottom ? INSPECTOR_BOTTOM_MIN : INSPECTOR_MIN
  const inspectorMax = useCallback(
    () => (inspectorBottom ? maxInspectorHeight() : maxInspectorWidth()),
    [inspectorBottom],
  )

  /** Size to open at: the user's own size if they set one, else the default. */
  const inspectorOpenSize = useCallback(() => {
    const placement = inspectorBottom ? 'bottom' : 'right'
    const current = layoutRef.current
    if (!current.inspectorWidthPinned) return openInspectorPaneSize(placement)
    return Math.round(clampSize(current.inspectorWidth, inspectorMin, Math.max(inspectorMin, inspectorMax())))
  }, [inspectorBottom, inspectorMax, inspectorMin])

  const openInspectorExpanded = useCallback(() => {
    if (!showInspector) return
    patch({ inspectorCollapsed: false, inspectorWidth: inspectorOpenSize() })
  }, [inspectorOpenSize, patch, showInspector])

  /** Reveal the inspector for a selection; no-op when it is already open. */
  const expandInspector = useCallback(() => {
    if (!showInspector || !layoutRef.current.inspectorCollapsed) return
    openInspectorExpanded()
  }, [openInspectorExpanded, showInspector])

  const explorerCollapsedRef = useRef(explorerCollapsed)
  explorerCollapsedRef.current = explorerCollapsed

  const fitExplorerWidth = useCallback((width) => {
    if (width == null || explorerCollapsedRef.current) return
    const next = Math.round(clampSize(width, EXPLORER_MIN, Math.max(EXPLORER_MIN, maxExplorerWidth())))
    if (next !== layoutRef.current.explorerWidth) setExplorerWidth(next)
  }, [setExplorerWidth])

  /** Record the latest label-fit width; apply it unless the user sized the explorer. */
  const applyExplorerAutoWidth = useCallback((width) => {
    explorerAutoWidthRef.current = width
    if (!layoutRef.current.explorerWidthPinned) fitExplorerWidth(width)
  }, [fitExplorerWidth])

  const refitExplorerPane = useCallback(() => {
    patch({ explorerWidthPinned: false })
    fitExplorerWidth(explorerAutoWidthRef.current)
  }, [fitExplorerWidth, patch])

  const collapseInspector = useCallback(() => {
    if (!showInspector) return
    patch({ inspectorCollapsed: true })
  }, [patch, showInspector])

  const shellInspector = useMemo(
    () => ({
      expandInspector,
      toggleInspector,
      inspectorCollapsed: inspectorLayoutCollapsed,
      inspectorBottom,
      onOpenPodLogs,
      applyExplorerAutoWidth,
      explorerAutoFit: !layout.explorerWidthPinned && !explorerCollapsed,
      collapseInspector,
      openInspectorExpanded,
    }),
    [
      expandInspector,
      toggleInspector,
      inspectorLayoutCollapsed,
      inspectorBottom,
      onOpenPodLogs,
      applyExplorerAutoWidth,
      layout.explorerWidthPinned,
      explorerCollapsed,
      collapseInspector,
      openInspectorExpanded,
    ],
  )

  useEffect(() => {
    if (!showInspector || layout.inspectorCollapsed) return undefined
    function onPointerDown(ev) {
      if (shouldDismissInspector(ev.target)) collapseInspector()
    }
    document.addEventListener('pointerdown', onPointerDown, true)
    return () => document.removeEventListener('pointerdown', onPointerDown, true)
  }, [collapseInspector, layout.inspectorCollapsed, showInspector])

  // Keep persisted pane sizes within the current window (e.g. after shrinking it).
  useEffect(() => {
    function clampPanesToViewport() {
      const maxInspector = inspectorMax()
      if (layout.inspectorWidth > maxInspector) setInspectorWidth(Math.max(inspectorMin, maxInspector))
      const maxExplorer = Math.max(EXPLORER_MIN, maxExplorerWidth())
      if (layout.explorerWidth > maxExplorer) setExplorerWidth(maxExplorer)
    }
    clampPanesToViewport()
    window.addEventListener('resize', clampPanesToViewport)
    return () => window.removeEventListener('resize', clampPanesToViewport)
  }, [
    inspectorMax,
    inspectorMin,
    layout.explorerWidth,
    layout.inspectorWidth,
    setExplorerWidth,
    setInspectorWidth,
  ])

  const startExplorerResize = useCallback((e) => {
    patch({ explorerWidthPinned: true })
    runPointerColResize(e, {
      startX: e.clientX,
      startSize: layout.explorerWidth,
      min: EXPLORER_MIN,
      max: Math.max(EXPLORER_MIN, maxExplorerWidth()),
      onSize: setExplorerWidth,
    })
  }, [layout.explorerWidth, patch, setExplorerWidth])

  const startInspectorResize = useCallback((e) => {
    patch({ inspectorWidthPinned: true })
    const opts = {
      startSize: layout.inspectorWidth,
      min: inspectorMin,
      max: Math.max(inspectorMin, inspectorMax()),
      invert: true,
      onSize: setInspectorWidth,
    }
    if (inspectorBottom) {
      runPointerRowResize(e, { ...opts, startY: e.clientY })
    } else {
      runPointerColResize(e, { ...opts, startX: e.clientX })
    }
  }, [
    inspectorBottom,
    inspectorMax,
    inspectorMin,
    layout.inspectorWidth,
    patch,
    setInspectorWidth,
  ])

  /** Shutter (collapsed inspector edge): click opens at default size, drag pulls the edge open. */
  const startInspectorShutter = useCallback((e) => {
    const startX = e.clientX
    const startY = e.clientY
    const max = Math.max(inspectorMin, inspectorMax())
    let dragging = false

    runPointerDragResize(e, {
      axis: inspectorBottom ? 'y' : 'x',
      onMove: (ev) => {
        if (!dragging) {
          if (Math.hypot(ev.clientX - startX, ev.clientY - startY) < SHUTTER_DRAG_THRESHOLD) return
          dragging = true
        }
        // Panel starts at zero size, so its edge tracks the pointer.
        const pulled = inspectorBottom ? startY - ev.clientY : startX - ev.clientX
        patch({
          inspectorCollapsed: false,
          inspectorWidthPinned: true,
          inspectorWidth: Math.round(clampSize(pulled, inspectorMin, max)),
        })
      },
      onEnd: ({ cancelled }) => {
        // A cancelled gesture (OS took the pointer) is not a click.
        if (dragging || cancelled) return
        openInspectorExpanded()
      },
    })
  }, [
    inspectorBottom,
    inspectorMax,
    inspectorMin,
    openInspectorExpanded,
    patch,
  ])

  /** Double-click the inspector edge: forget the user's size, back to default. */
  const refitInspectorPane = useCallback(() => {
    patch({
      inspectorWidthPinned: false,
      inspectorWidth: openInspectorPaneSize(inspectorBottom ? 'bottom' : 'right'),
    })
  }, [inspectorBottom, patch])

  const relations = graphRelations || defaultRelations()

  const inspectorStyle = inspectorBottom
    ? {
      height: `${layout.inspectorWidth}px`,
      minHeight: `${INSPECTOR_BOTTOM_MIN}px`,
      maxHeight: `${maxInspectorHeight()}px`,
    }
    : { width: `${layout.inspectorWidth}px`, minWidth: `${layout.inspectorWidth}px` }

  const inspectorPanel = showInspector && !inspectorLayoutCollapsed && (
    <>
      <div
        className={`pane-resize-handle ${inspectorBottom ? 'pane-resize-handle-row' : ''}`}
        role="separator"
        aria-orientation={inspectorBottom ? 'horizontal' : 'vertical'}
        aria-label="Resize inspector"
        title="Drag to resize · double-click to reset size"
        onPointerDown={startInspectorResize}
        onDoubleClick={refitInspectorPane}
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
          collapsed={railCollapsed}
          onToggleCollapse={toggleRail}
        />

        <div className="app-shell-main">
          <div className={`app-shell-panes ${inspectorBottom ? 'inspector-bottom' : 'inspector-right'}`}>
            {hasExplorer && (
              <>
                <ContextExplorer
                  tab={tab}
                  collapsed={explorerCollapsed}
                  onToggleCollapse={handleToggleExplorer}
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
                    title="Drag to resize explorer · double-click to auto-fit labels"
                    onPointerDown={startExplorerResize}
                    onDoubleClick={refitExplorerPane}
                  />
                )}
              </>
            )}

            <main className="app-workspace">
              {children}
            </main>

            {!inspectorBottom && showInspector && inspectorLayoutCollapsed && (
              <div
                className="app-inspector-shutter"
                role="separator"
                aria-orientation="vertical"
                aria-label="Open and resize inspector"
                title="Click to open · drag to resize"
                onPointerDown={startInspectorShutter}
              />
            )}
            {!inspectorBottom && inspectorPanel}
          </div>

          {inspectorBottom && showInspector && inspectorLayoutCollapsed && (
            <div
              className="app-inspector-shutter app-inspector-shutter-bottom"
              role="separator"
              aria-orientation="horizontal"
              aria-label="Open and resize inspector"
              title="Click to open · drag to resize"
              onPointerDown={startInspectorShutter}
            />
          )}
          {inspectorBottom && inspectorPanel}
        </div>
      </div>
    </div>
    </ShellInspectorProvider>
  )
}
