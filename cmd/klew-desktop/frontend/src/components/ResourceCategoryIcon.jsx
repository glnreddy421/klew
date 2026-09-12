import {
  isWorkloadsCategory,
  resourceCategoryIconName,
  resourceCategoryToneClass,
  WORKLOADS_ICON_PATHS,
} from '../lib/resourceCategoryIcons'

export function ResourceCategoryIcon({
  categoryId,
  size = 16,
  className = '',
  title,
}) {
  const px = typeof size === 'number' ? size : 16
  const label = title || categoryId || 'Category'
  const toneClass = resourceCategoryToneClass(categoryId)

  if (isWorkloadsCategory(categoryId)) {
    return (
      <span
        className={`resource-category-icon ${toneClass} ${className}`.trim()}
        title={label}
        aria-hidden="true"
      >
        <svg viewBox="0 0 24 24" width={px} height={px} fill="currentColor" focusable="false">
          {WORKLOADS_ICON_PATHS.map((d) => (
            <path key={d} d={d} />
          ))}
        </svg>
      </span>
    )
  }

  const iconName = resourceCategoryIconName(categoryId)

  return (
    <span
      className={`resource-category-icon material-symbols-outlined ${toneClass} ${className}`.trim()}
      title={label}
      aria-hidden="true"
      style={{ fontSize: px, width: px, height: px }}
    >
      {iconName}
    </span>
  )
}
