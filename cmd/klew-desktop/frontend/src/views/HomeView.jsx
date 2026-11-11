import { useMemo, useRef, useState } from 'react'
import { ClusterHomeList } from '../components/ClusterHomeList.jsx'
import { BrailleWordmark } from '../components/BrailleWordmark.jsx'
import { KlewCliButton } from '../components/shell/KlewCliButton.jsx'
import { WHATS_NEW_VERSION, WhatsNewBanner } from '../components/WhatsNewBanner.jsx'
import {
  collectContextTags,
  filterContextsForHome,
  normalizeTag,
  orderContextsForDisplay,
} from '../lib/clusterContexts.js'

/**
 * Klew home — full-width contexts table from kubeconfig.
 * Choosing a context opens Resources for that cluster.
 */
export function HomeView({
  cluster,
  clusterStatus,
  connection,
  syncing = false,
  connecting = false,
  statusLoading = false,
  reconnectBusy = false,
  monitoringPaused = false,
  defaultContext = '',
  favoriteContexts = [],
  connectedContexts = [],
  contextTags = {},
  contextAliases = {},
  contextDetails = {},
  onSelectCluster,
  onSetDefaultContext,
  onToggleFavoriteContext,
  onAddContextTag,
  onRemoveContextTag,
  onRenameContext,
  onReconnect,
  onDisconnect,
  onOpenProxySettings,
  onOpenMcpSetup,
  welcomeSeenVersion = '',
  homeLayout = 'tiles',
  onPrefsChange,
}) {
  const cliRef = useRef(null)
  const showWelcome = Boolean(onPrefsChange) && welcomeSeenVersion !== WHATS_NEW_VERSION
  const openCli = () => cliRef.current?.querySelector('button')?.click()
  const [query, setQuery] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [sort, setSort] = useState({ columnId: 'name', direction: 'asc' })
  const contexts = cluster?.contexts || []
  const knownTags = useMemo(
    () => collectContextTags(contextTags, contexts.map((c) => c.name)),
    [contextTags, contexts],
  )
  const filteredContexts = useMemo(
    () => filterContextsForHome(contexts, {
      favoriteContexts,
      contextTags,
      contextAliases,
      query,
      tag: tagFilter,
    }),
    [contexts, favoriteContexts, contextTags, contextAliases, query, tagFilter],
  )
  const displayContexts = useMemo(
    () => sortContexts(filteredContexts, sort, {
      favoriteContexts,
      defaultContext,
      contextTags,
    }),
    [filteredContexts, sort, favoriteContexts, defaultContext, contextTags],
  )
  const filtering = Boolean(String(query || '').trim() || tagFilter)
  const activeTag = normalizeTag(tagFilter)

  function handleSort(columnId) {
    setSort((prev) => (
      prev.columnId === columnId
        ? { columnId, direction: prev.direction === 'asc' ? 'desc' : 'asc' }
        : { columnId, direction: 'asc' }
    ))
  }

  const tiles = homeLayout !== 'list'
  const shown = tiles
    ? orderContextsForDisplay(filteredContexts, { defaultContext, favoriteContexts })
    : displayContexts

  function onSearchKeyDown(e) {
    if (e.key === 'Enter' && shown.length > 0 && String(query || '').trim()) {
      e.preventDefault()
      onSelectCluster?.(shown[0].name)
    } else if (e.key === 'Escape' && query) {
      e.preventDefault()
      setQuery('')
    }
  }

  return (
    <section
      className={['entity-table', 'klew-contexts', 'klew-start', tiles ? 'is-tiles' : 'is-list'].join(' ')}
      aria-labelledby="klew-home-clusters-title"
    >
      {showWelcome && (
        <WhatsNewBanner
          onDismiss={() => onPrefsChange?.({ welcomeSeenVersion: WHATS_NEW_VERSION })}
          onOpenMcpSetup={onOpenMcpSetup}
          onOpenCli={openCli}
        />
      )}
      <nav className="klew-start-tools" aria-label="Klew tools">
        <span className="klew-cli-slot" ref={cliRef}>
          <KlewCliButton
            contextName={cluster?.selectedContext || cluster?.currentContext || ''}
            namespace={cluster?.selectedNamespace || ''}
            buttonClassName="klew-start-tool"
          >
            <TerminalIcon />
            <span>CLI</span>
          </KlewCliButton>
        </span>
        {onOpenMcpSetup && (
          <button type="button" className="klew-start-tool" onClick={onOpenMcpSetup} title="Set up the MCP server for AI assistants">
            <ComputerIcon />
            <span>MCP</span>
          </button>
        )}
      </nav>
      <header className="klew-start-hero">
        <h1 id="klew-home-clusters-title" className="sr-only">Contexts</h1>
        <BrailleWordmark text="KLEW" scale={1} pitch={5.5} cellGap={2.6} radius={1.9} className="klew-start-mark" />
        <label className="klew-start-search">
          <svg className="klew-start-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <circle cx="11" cy="11" r="6.5" />
            <path d="M20 20l-4-4" strokeLinecap="round" />
          </svg>
          <input
            type="search"
            className="klew-start-search-input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onSearchKeyDown}
            placeholder="Search or open a context…"
            aria-label="Search contexts"
            spellCheck={false}
            autoComplete="off"
            autoCorrect="off"
            autoCapitalize="off"
          />
          {String(query || '').trim() && shown.length > 0 && (
            <span className="klew-start-search-hint" aria-hidden="true">↵ {shown[0].name}</span>
          )}
        </label>
        <div className="klew-start-bar">
          <div className="klew-home-tag-filters" role="toolbar" aria-label="Filter by tag">
            {knownTags.length > 0 && (
              <>
                <button
                  type="button"
                  className={['klew-home-tag-chip', !activeTag ? 'is-active' : ''].filter(Boolean).join(' ')}
                  aria-pressed={!activeTag}
                  onClick={() => setTagFilter('')}
                >
                  All
                </button>
                {knownTags.map((tag) => (
                  <button
                    key={tag}
                    type="button"
                    className={['klew-home-tag-chip', activeTag === tag ? 'is-active' : ''].filter(Boolean).join(' ')}
                    aria-pressed={activeTag === tag}
                    onClick={() => setTagFilter(activeTag === tag ? '' : tag)}
                  >
                    {tag}
                  </button>
                ))}
              </>
            )}
            <span className="klew-home-count">
              {filtering ? `${shown.length} of ${contexts.length}` : contexts.length} context{contexts.length === 1 ? '' : 's'}
            </span>
          </div>
          <div className="klew-start-view" role="group" aria-label="Layout">
            <button
              type="button"
              className={tiles ? 'is-active' : ''}
              aria-pressed={tiles}
              onClick={() => onPrefsChange?.({ homeLayout: 'tiles' })}
            >
              Tiles
            </button>
            <button
              type="button"
              className={!tiles ? 'is-active' : ''}
              aria-pressed={!tiles}
              onClick={() => onPrefsChange?.({ homeLayout: 'list' })}
            >
              List
            </button>
          </div>
        </div>
      </header>

      <ClusterHomeList
        cluster={cluster}
        clusterStatus={clusterStatus}
        connection={connection}
        syncing={syncing}
        connecting={connecting}
        statusLoading={statusLoading}
        reconnectBusy={reconnectBusy}
        monitoringPaused={monitoringPaused}
        defaultContext={defaultContext}
        favoriteContexts={favoriteContexts}
        connectedContexts={connectedContexts}
        contextTags={contextTags}
        contextAliases={contextAliases}
        contextDetails={contextDetails}
        displayContexts={shown}
        filtering={filtering}
        sort={sort}
        onSort={handleSort}
        onSelectCluster={onSelectCluster}
        onSetDefaultContext={onSetDefaultContext}
        onToggleFavoriteContext={onToggleFavoriteContext}
        onAddContextTag={onAddContextTag}
        onRemoveContextTag={onRemoveContextTag}
        onRenameContext={onRenameContext}
        onReconnect={onReconnect}
        onDisconnect={onDisconnect}
        onOpenProxySettings={onOpenProxySettings}
        layout={tiles ? 'tiles' : 'list'}
      />

    </section>
  )
}

/** Terminal window with a prompt: the klew command line. */
function TerminalIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="4.5" width="18" height="15" rx="2.5" />
      <path d="M7.5 9.5l3 2.5-3 2.5" />
      <path d="M12.5 15h4" />
    </svg>
  )
}

/** Desktop computer: the MCP server that runs on this machine for AI clients. */
function ComputerIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="4" width="18" height="12" rx="2" />
      <path d="M9 20h6" />
      <path d="M12 16v4" />
    </svg>
  )
}

function sortContexts(contexts, sort, { favoriteContexts = [], defaultContext = '', contextTags = {} } = {}) {
  const list = Array.isArray(contexts) ? [...contexts] : []
  const columnId = sort?.columnId || 'name'
  const direction = sort?.direction === 'desc' ? -1 : 1
  const favSet = new Set(favoriteContexts || [])

  list.sort((a, b) => {
    const cmp = compareContextField(a, b, columnId, {
      favoriteContexts: favSet,
      defaultContext,
      contextTags,
    })
    if (cmp !== 0) return cmp * direction
    return String(a.name || '').localeCompare(String(b.name || ''))
  })
  return list
}

function compareContextField(a, b, columnId, { favoriteContexts, defaultContext, contextTags }) {
  switch (columnId) {
    case 'cluster':
      return String(a.cluster || '').localeCompare(String(b.cluster || ''))
    case 'user':
      return String(a.user || '').localeCompare(String(b.user || ''))
    case 'namespace':
      return String(a.namespace || '').localeCompare(String(b.namespace || ''))
    case 'tags': {
      const at = (contextTags?.[a.name] || []).join(',')
      const bt = (contextTags?.[b.name] || []).join(',')
      return at.localeCompare(bt)
    }
    case 'favorite': {
      const af = favoriteContexts.has(a.name) ? 1 : 0
      const bf = favoriteContexts.has(b.name) ? 1 : 0
      return bf - af
    }
    case 'default': {
      const ad = a.name === defaultContext ? 1 : 0
      const bd = b.name === defaultContext ? 1 : 0
      return bd - ad
    }
    case 'name':
    default:
      return String(a.name || '').localeCompare(String(b.name || ''))
  }
}
