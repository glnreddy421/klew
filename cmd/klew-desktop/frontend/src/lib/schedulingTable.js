export const SCHEDULING_TABLES = {
  nodeSelector: {
    title: 'Node Selector',
    columns: ['Key', 'Value'],
    pickRows: (sched) => (sched?.nodeSelector || []).map((row) => [row.key, row.value]),
  },
  tolerations: {
    title: 'Tolerations',
    columns: ['Key', 'Operator', 'Value', 'Effect', 'Seconds'],
    pickRows: (sched) => (sched?.tolerations || []).map((row) => [
      row.key,
      row.operator || '',
      row.value || '',
      row.effect || '',
      row.seconds || '',
    ]),
  },
  affinity: {
    title: 'Affinity',
    columns: ['Type', 'Weight', 'Topology', 'Namespaces', 'Match'],
    pickRows: (sched) => (sched?.affinity || []).map((row) => [
      row.type,
      row.weight || '',
      row.topology || '',
      row.namespaces || '',
      row.match || '',
    ]),
  },
  taints: {
    title: 'Taints',
    columns: ['Key', 'Value', 'Effect'],
    pickRows: (sched) => (sched?.taints || []).map((row) => [
      row.key,
      row.value || '',
      row.effect || '',
    ]),
  },
}

export function schedulingTableForRow(row, columnId) {
  const def = SCHEDULING_TABLES[columnId]
  if (!def) return null
  const sched = row?.scheduling || null
  const rows = def.pickRows(sched).filter((cells) => cells.some((cell) => String(cell || '').trim() && cell !== '—'))
  if (!rows.length) return null
  const count = rows.length
  const unit = columnId === 'nodeSelector' ? (count === 1 ? 'key' : 'keys') : (count === 1 ? 'rule' : 'rules')
  return {
    title: def.title,
    columns: def.columns,
    rows,
    summary: `${count} ${unit}`,
  }
}
