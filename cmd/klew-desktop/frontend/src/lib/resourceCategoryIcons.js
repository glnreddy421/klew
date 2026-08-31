/**
 * Resource navigator category icons — Material-style glyphs aligned with Lens/Freelens.
 * @see https://fonts.google.com/icons
 */

/** @typedef {'workloads'|'network'|'storage'|'config'|'security'|'cluster'|'custom'|'other'} ResourceCategoryId */

export const RESOURCE_CATEGORY_ICON_NAMES = {
  workloads: 'workloads',
  network: 'device_hub',
  storage: 'storage',
  config: 'tune',
  security: 'shield',
  cluster: 'hub',
  custom: 'extension',
  other: 'more_horiz',
}

/** Lens/Freelens workloads glyph (pod cluster). */
export const WORKLOADS_ICON_PATHS = [
  'M12 7.47 16.25 5 12 2.56 7.75 5z',
  'M11.66 8.06 7.41 5.6v4.91l4.25 2.49z',
  'M12.34 8.06v4.94l4.25-2.45V5.6z',
  'M16.93 15.94 21.19 13.49 16.93 11 12.68 13.45z',
  'M16.59 16.53 12.34 14.08v4.92l4.25 2.46z',
  'M17.26 16.53v4.91l4.26-2.44v-4.93z',
  'M7.07 15.94 11.32 13.49 7.07 11 2.82 13.49z',
  'M6.74 16.53 2.48 14.07v4.93l4.26 2.46z',
  'M7.41 16.53v4.91l4.25-2.44v-4.93z',
]

export function resourceCategoryIconName(categoryId) {
  return RESOURCE_CATEGORY_ICON_NAMES[categoryId] || RESOURCE_CATEGORY_ICON_NAMES.other
}

export function isWorkloadsCategory(categoryId) {
  return categoryId === 'workloads'
}
