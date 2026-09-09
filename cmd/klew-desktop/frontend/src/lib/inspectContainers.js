/** Parse inspector container-group sections into a structured pod container model. */

function tableRows(section) {
  const cols = section?.table?.columns || []
  const rows = section?.table?.rows || []
  return rows.map((row) => {
    const out = {}
    cols.forEach((col, i) => {
      out[col] = row[i] ?? ''
    })
    return out
  })
}

function sectionById(sections, id) {
  return (sections || []).find((s) => s.id === id) || null
}

function ensureContainer(map, name, init = false) {
  const key = `${init ? 'init:' : ''}${name}`
  if (!map.has(key)) {
    map.set(key, { name, init, state: {}, spec: {}, resources: {}, mounts: [], statusMounts: [], probes: [], security: {}, env: [], secretEnv: [] })
  }
  return map.get(key)
}

export function parseInspectContainers(sections = []) {
  const byName = new Map()
  const podLevel = { imagePullSecrets: [], restartHistory: [], sidecars: [], fallback: [] }

  const stateSec = sectionById(sections, 'containerStates')
  for (const row of tableRows(stateSec)) {
    const c = ensureContainer(byName, row.Name, false)
    c.state = {
      ready: row.Ready,
      restarts: row.Restarts,
      state: row.State,
      reason: row.Reason,
      exit: row.Exit,
      image: row.Image,
      imageId: row['Image ID'],
      containerId: row['Container ID'],
      started: row.Started,
      allocCpu: row['Alloc CPU'],
      allocMem: row['Alloc Mem'],
    }
    if (row.Image && !c.spec.image) c.spec.image = row.Image
  }

  const initStateSec = sectionById(sections, 'initContainerStates')
  for (const row of tableRows(initStateSec)) {
    const c = ensureContainer(byName, row.Name, true)
    c.state = {
      ready: row.Ready,
      restarts: row.Restarts,
      state: row.State,
      reason: row.Reason,
      exit: row.Exit,
      image: row.Image,
      imageId: row['Image ID'],
      containerId: row['Container ID'],
      started: row.Started,
    }
    if (row.Image && !c.spec.image) c.spec.image = row.Image
  }

  for (const row of tableRows(sectionById(sections, 'containers'))) {
    const c = ensureContainer(byName, row.Name, false)
    c.resources = {
      reqCpu: row['Req CPU'],
      reqMem: row['Req Mem'],
      limCpu: row['Lim CPU'],
      limMem: row['Lim Mem'],
      ports: row.Ports,
    }
    if (row.Image && !c.spec.image) c.spec.image = row.Image
  }

  for (const row of tableRows(sectionById(sections, 'initContainers'))) {
    const c = ensureContainer(byName, row.Name, true)
    c.resources = {
      reqCpu: row['Req CPU'],
      reqMem: row['Req Mem'],
      limCpu: row['Lim CPU'],
      limMem: row['Lim Mem'],
      ports: row.Ports,
    }
    if (row.Image && !c.spec.image) c.spec.image = row.Image
  }

  for (const row of tableRows(sectionById(sections, 'containerSpec'))) {
    const init = byName.has(`init:${row.Name}`)
    const c = ensureContainer(byName, row.Name, init)
    c.spec = {
      ...c.spec,
      image: row.Image || c.spec.image,
      pullPolicy: row['Pull Policy'],
      command: row.Command,
      args: row.Args,
      workingDir: row['Working Dir'],
    }
  }

  for (const row of tableRows(sectionById(sections, 'resources'))) {
    const init = byName.has(`init:${row.Name}`)
    const c = ensureContainer(byName, row.Container || row.Name, init)
    c.resources = {
      ...c.resources,
      reqCpu: row['Req CPU'] || c.resources.reqCpu,
      reqMem: row['Req Mem'] || c.resources.reqMem,
      limCpu: row['Lim CPU'] || c.resources.limCpu,
      limMem: row['Lim Mem'] || c.resources.limMem,
    }
  }

  for (const row of tableRows(sectionById(sections, 'volumeMounts'))) {
    const init = byName.has(`init:${row.Container}`)
    const c = ensureContainer(byName, row.Container, init)
    c.mounts.push({
      volume: row.Volume,
      mountPath: row['Mount Path'],
      subPath: row['Sub Path'],
      readOnly: row['Read Only'],
    })
  }

  for (const row of tableRows(sectionById(sections, 'containerVolumeMounts'))) {
    const init = byName.has(`init:${row.Container}`)
    const c = ensureContainer(byName, row.Container, init)
    c.statusMounts.push({
      volume: row.Volume,
      mountPath: row['Mount Path'],
      readOnly: row['Read Only'],
      recursiveRO: row['Recursive RO'],
    })
  }

  for (const row of tableRows(sectionById(sections, 'probes'))) {
    const c = ensureContainer(byName, row.Container, false)
    c.probes.push({
      type: row.Probe,
      probeKind: row.Type,
      target: row.Target,
      initialDelay: row['Initial Delay'],
      period: row.Period,
      timeout: row.Timeout,
      failures: row.Failures,
    })
  }

  for (const row of tableRows(sectionById(sections, 'securityContext'))) {
    const c = ensureContainer(byName, row.Container, false)
    c.security = {
      runAsUser: row['Run As User'],
      runAsGroup: row['Run As Group'],
      privileged: row.Privileged,
      readOnlyRootFs: row['Read Only Root FS'],
    }
  }

  for (const row of tableRows(sectionById(sections, 'environment'))) {
    const c = ensureContainer(byName, row.Container, false)
    c.env.push({ name: row.Name, source: row.Source, value: row.Value })
  }

  const secretSec = sectionById(sections, 'resolvedSecretEnv')
  for (const row of tableRows(secretSec)) {
    const c = ensureContainer(byName, row.Container, false)
    c.secretEnv.push({
      name: row.Name,
      secretKey: row['Secret Key'],
      value: row.Value,
      sensitive: true,
    })
  }
  if (secretSec?.notes?.length) {
    podLevel.secretNotes = secretSec.notes
  }

  podLevel.imagePullSecrets = tableRows(sectionById(sections, 'imagePullSecrets')).map((r) => r.Name).filter(Boolean)
  podLevel.restartHistory = tableRows(sectionById(sections, 'restartHistory'))
  podLevel.sidecars = tableRows(sectionById(sections, 'sidecars'))

  const known = new Set([
    'containerStates', 'initContainerStates', 'containerVolumeMounts', 'restartHistory',
    'containers', 'containerSpec', 'resources', 'volumeMounts', 'probes', 'securityContext',
    'initContainers', 'sidecars', 'environment', 'resolvedSecretEnv', 'imagePullSecrets',
  ])
  podLevel.fallback = (sections || []).filter((s) => !known.has(s.id))

  const containers = [...byName.values()]
    .filter((c) => !c.init)
    .sort((a, b) => a.name.localeCompare(b.name))
  const initContainers = [...byName.values()]
    .filter((c) => c.init)
    .sort((a, b) => a.name.localeCompare(b.name))

  return { containers, initContainers, podLevel }
}

export function containerStateTone(state = '') {
  const s = String(state || '').toLowerCase()
  if (s === 'running') return 'running'
  if (s === 'waiting') return 'waiting'
  if (s === 'terminated') return 'terminated'
  return 'unknown'
}

export function hasContainerPanelData(model) {
  if (!model) return false
  return model.containers.length > 0 || model.initContainers.length > 0 || model.podLevel.fallback.length > 0
}
