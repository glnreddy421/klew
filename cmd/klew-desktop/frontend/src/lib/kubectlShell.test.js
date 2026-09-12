import { describe, expect, it } from 'vitest'
import { kubeconfigForCommandFlag } from './kubectlShell.js'

describe('kubeconfigForCommandFlag', () => {
  it('returns empty for default kubeconfig locations', () => {
    expect(kubeconfigForCommandFlag('/Users/gln/.kube/config')).toBe('')
    expect(kubeconfigForCommandFlag('~/.kube/config')).toBe('')
    expect(kubeconfigForCommandFlag('')).toBe('')
  })

  it('returns custom paths unchanged', () => {
    expect(kubeconfigForCommandFlag('/work/clusters/prod.yaml')).toBe('/work/clusters/prod.yaml')
  })
})
