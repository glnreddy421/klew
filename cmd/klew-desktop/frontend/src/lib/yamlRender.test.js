import { afterEach, describe, expect, it } from 'vitest'
import {
  clearYamlRenderCache,
  getYamlLineCount,
  getYamlRenderMeta,
  shouldHighlightYaml,
  yamlContentKey,
  YAML_HIGHLIGHT_MAX_LINES,
} from './yamlRender.js'

afterEach(() => {
  clearYamlRenderCache()
})

describe('yamlRender', () => {
  it('builds stable content keys', () => {
    expect(yamlContentKey('apiVersion: v1')).toBe(yamlContentKey('apiVersion: v1'))
    expect(yamlContentKey('apiVersion: v1')).not.toBe(yamlContentKey('apiVersion: v2'))
  })

  it('caches line splits across calls', () => {
    const text = 'a\nb\nc'
    expect(getYamlLineCount(text)).toBe(3)
    expect(getYamlLineCount(text)).toBe(3)
    expect(getYamlRenderMeta(text).lines).toEqual(['a', 'b', 'c'])
  })

  it('uses plain mode for large manifests', () => {
    const text = Array.from({ length: YAML_HIGHLIGHT_MAX_LINES + 1 }, (_, i) => `key${i}: value`).join('\n')
    expect(shouldHighlightYaml(getYamlLineCount(text))).toBe(false)
    expect(getYamlRenderMeta(text).mode).toBe('plain')
  })

  it('uses highlight mode for small manifests', () => {
    const text = 'apiVersion: v1\nkind: Pod'
    expect(getYamlRenderMeta(text).mode).toBe('highlight')
  })
})
