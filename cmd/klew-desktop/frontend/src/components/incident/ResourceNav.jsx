import { getKindCountDisplay, resourceMetadataTitle } from '../../lib/resourceCatalog.js'
import { categorySupportsOverview, isCategoryOverview } from '../../lib/resourceNavigation.js'
import { ResourceCategoryIcon } from '../ResourceCategoryIcon.jsx'

function Chevron({ open }) {
  return (
    <span className={`scope-chevron ${open ? 'open' : ''}`} aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2">
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
  const overviewSelected = activeInGroup && isCategoryOverview(selectedKind)
  const categoryCount = category.kinds.reduce((sum, k) => {
    const d = countLabel(k, countsLoading)
    if (d.className === 'count-active' && d.label && d.label !== '…') return sum + Number(d.label)
    return sum
  }, 0)

  return (
    <div className={`resource-nav-group ${expanded ? 'is-open' : ''}`}>
      <button
        type="button"
        className="resource-nav-group-row"
        aria-expanded={expanded}
        onClick={onToggle}
      >
        <Chevron open={expanded} />
        <ResourceCategoryIcon categoryId={category.id} size={16} title={category.label} />
        <span className="resource-nav-group-label">{category.label}</span>
        {categoryCount > 0 && (
          <span className="resource-nav-group-count">{categoryCount}</span>
        )}
      </button>
      {expanded && (
        <ul className="resource-nav-kinds">
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
      )}
    </div>
  )
}

/**
 * Compact resource-kind picker — categories expand, kinds select.
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
    <nav className="resource-nav" aria-label="Resource kinds">
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
