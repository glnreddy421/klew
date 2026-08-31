import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import {
  isWorkloadsCategory,
  resourceCategoryIconName,
  RESOURCE_CATEGORY_ICON_NAMES,
} from './resourceCategoryIcons.js'

describe('resourceCategoryIcons', () => {
  it('maps all navigator categories', () => {
    for (const id of [
      'workloads', 'network', 'storage', 'config', 'security', 'cluster', 'custom', 'other',
    ]) {
      assert.ok(RESOURCE_CATEGORY_ICON_NAMES[id], id)
      assert.ok(resourceCategoryIconName(id))
    }
  })

  it('only workloads uses the custom svg glyph', () => {
    assert.equal(isWorkloadsCategory('workloads'), true)
    assert.equal(isWorkloadsCategory('network'), false)
    assert.equal(resourceCategoryIconName('network'), 'device_hub')
  })

  it('falls back for unknown categories', () => {
    assert.equal(resourceCategoryIconName('unknown'), 'more_horiz')
  })
})
