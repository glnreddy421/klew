import { describe, expect, it } from 'vitest'
import { schedulingTableForRow } from './schedulingTable.js'

describe('schedulingTableForRow', () => {
  it('maps structured tolerations to table rows', () => {
    const table = schedulingTableForRow({
      scheduling: {
        tolerations: [{
          key: 'node.kubernetes.io/not-ready',
          operator: 'Exists',
          effect: 'NoExecute',
          seconds: '300',
        }],
      },
    }, 'tolerations')

    expect(table?.rows).toEqual([[
      'node.kubernetes.io/not-ready',
      'Exists',
      '',
      'NoExecute',
      '300',
    ]])
    expect(table?.summary).toBe('1 rule')
  })

  it('maps node taints for node rows', () => {
    const table = schedulingTableForRow({
      scheduling: {
        taints: [{ key: 'dedicated', value: 'gpu', effect: 'NoSchedule' }],
      },
    }, 'taints')

    expect(table?.rows[0][0]).toBe('dedicated')
  })
})
