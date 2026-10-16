import { useEffect, useId, useRef, useState } from 'react'
import { getKindCountDisplay, resourceMetadataTitle } from '../../lib/resourceCatalog.js'
import { categorySupportsOverview, isCategoryOverview } from '../../lib/resourceNavigation.js'
import { resourceCategoryToneClass } from '../../lib/resourceCategoryIcons.js'
import { ResourceCategoryIcon } from '../ResourceCategoryIcon.jsx'

function Chevron({ open }) {
  return (
    <span className={`scope-chevron resource-nav-chevron ${open ? 'open' : ''}`} aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </span>
  )
}

function countLabel(kindGroup, countsLoading) {
  const count = getKindCountDisplay(kindGroup)
  if (countsLoading && count.className === 'count-unknown' && !count.label) {
    return { ...count, label: '…' }
  }
  return count
}

function ResourceKindList({
  category,
  selectedGroupId,
  selectedKind,
  selectedResourceId,
  onSelectKind,
  onSelectOverview,
  countsLoading = false,
}) {
  const activeInGroup = selectedGroupId === category.id
  const overviewSelected = activeInGroup && isCategoryOverview(selectedKind)
  const toneClass = resourceCategoryToneClass(category.id)

  return (
    <ul className={`resource-nav-kinds ${toneClass}`}>
      {categorySupportsOverview(category.id) && (
        <li>
          <button
            type="button"
            className={[
              'resource-nav-kind-row',
              'resource-nav-overview-row',
              overviewSelected ? 'selected' : '',
            ].filter(Boolean).join(' ')}
            aria-selected={overviewSelected}
            onClick={() => onSelectOverview?.(category.id)}
          >
            {overviewSelected && <span className="resource-nav-kind-indicator" aria-hidden="true" />}
            <span className="resource-nav-kind-label">Overview</span>
          </button>
        </li>
      )}
      {category.kinds.map((kindGroup) => {
        const selected = activeInGroup
          && !overviewSelected
          && selectedKind === kindGroup.kind
          && (!selectedResourceId || selectedResourceId === kindGroup.resourceId)
        const count = countLabel(kindGroup, countsLoading)
        return (
          <li key={kindGroup.resourceId || kindGroup.kind}>
            <button
              type="button"
              className={[
                'resource-nav-kind-row',
                selected ? 'selected' : '',
                count.className,
              ].filter(Boolean).join(' ')}
              aria-selected={selected}
              title={resourceMetadataTitle(kindGroup)}
              onClick={() => onSelectKind(category.id, kindGroup.kind, kindGroup.resourceId)}
            >
              {selected && <span className="resource-nav-kind-indicator" aria-hidden="true" />}
              <span className="resource-nav-kind-label">
                {kindGroup.label}
                {kindGroup.legacy && (
                  <span className="resource-nav-legacy-tag">legacy</span>
                )}
              </span>
              <span
                className={`resource-nav-count ${count.className}`}
                title={count.title}
                aria-hidden={!count.label && count.className !== 'count-denied'}
              >
                {count.label}
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}

/** Matches the .resource-nav-panel grid-row transition (0.2s) plus a little slack. */
const PANEL_CLOSE_MS = 260

function ResourceGroupSection({
  category,
  expanded,
  onToggle,
  selectedGroupId,
  selectedKind,
  selectedResourceId,
  onSelectKind,
  onSelectOverview,
  countsLoading = false,
}) {
  const activeInGroup = selectedGroupId === category.id
  const categoryCount = category.kinds.reduce((sum, k) => {
    const d = countLabel(k, countsLoading)
    if (d.className === 'count-active' && d.label && d.label !== '…') return sum + Number(d.label)
    return sum
  }, 0)

  const toneClass = resourceCategoryToneClass(category.id)
  const panelId = useId()
  // Kind rows render only while open (plus the close animation), so collapsed
  // groups cost nothing on count refreshes — clusters can have hundreds of CRDs.
  const [closing, setClosing] = useState(false)
  const wasExpandedRef = useRef(expanded)
  useEffect(() => {
    const wasExpanded = wasExpandedRef.current
    wasExpandedRef.current = expanded
    if (expanded || !wasExpanded) {
      setClosing(false)
      return undefined
    }
    setClosing(true)
    const t = setTimeout(() => setClosing(false), PANEL_CLOSE_MS)
    return () => clearTimeout(t)
  }, [expanded])
  const renderKinds = expanded || closing

  return (
    <div className={[
      'resource-nav-group',
      toneClass,
      expanded ? 'is-open' : '',
      activeInGroup ? 'is-active' : '',
    ].filter(Boolean).join(' ')}>
      <button
        type="button"
        className={[
          'resource-nav-group-row',
          toneClass,
          activeInGroup ? 'is-active' : '',
        ].filter(Boolean).join(' ')}
        aria-expanded={expanded}
        aria-controls={panelId}
        onClick={onToggle}
      >
        <Chevron open={expanded} />
        <ResourceCategoryIcon categoryId={category.id} size={16} title={category.label} />
        <span className="resource-nav-group-label">{category.label}</span>
        {categoryCount > 0 && (
          <span className="resource-nav-group-count">{categoryCount}</span>
        )}
      </button>
      {/* Panel stays mounted so open/close can animate; inert keeps hidden rows out of tab order. */}
      <div
        id={panelId}
        className="resource-nav-panel"
        {...(expanded ? {} : { inert: '', 'aria-hidden': true })}
      >
        <div className="resource-nav-panel-inner">
          {renderKinds && (
          <ResourceKindList
            category={category}
            selectedGroupId={selectedGroupId}
            selectedKind={selectedKind}
            selectedResourceId={selectedResourceId}
            onSelectKind={onSelectKind}
            onSelectOverview={onSelectOverview}
            countsLoading={countsLoading}
          />
          )}
        </div>
      </div>
    </div>
  )
}

/**
 * Vertical resource-kind picker — accordion categories (one open at a time), kinds select.
 */
export function ResourceNav({
  categories = [],
  expandedGroups,
  selectedGroupId,
  selectedKind,
  selectedResourceId,
  onToggleGroup,
  onSelectKind,
  onSelectOverview,
  countsLoading = false,
}) {
  return (
    <nav className="resource-nav resource-nav-accordion" aria-label="Resource kinds">
      <div className="resource-nav-tree">
        {categories.map((cat) => (
          <ResourceGroupSection
            key={cat.id}
            category={cat}
            expanded={expandedGroups.has(cat.id)}
            onToggle={() => onToggleGroup(cat.id)}
            selectedGroupId={selectedGroupId}
            selectedKind={selectedKind}
            selectedResourceId={selectedResourceId}
            onSelectKind={onSelectKind}
            onSelectOverview={onSelectOverview}
            countsLoading={countsLoading}
          />
        ))}
        {!categories.length && (
          <p className="scope-empty muted">No resources in scope.</p>
        )}
      </div>
    </nav>
  )
}
