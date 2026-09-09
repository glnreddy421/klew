/** Extract Lens-style service endpoint fields from inspect groups or summary. */
export function findServiceEndpointFields(inspect) {
  if (!inspect || inspect.kind !== 'Service') return null

  const fromGroups = (inspect.groups || [])
    .flatMap((g) => g.sections || [])
    .find((s) => s.id === 'serviceEndpoint' || s.title === 'Endpoint')

  if (fromGroups?.fields?.length) {
    return fieldsToEndpoint(fromGroups.fields)
  }

  const summary = inspect.summary || []
  const endpoints = summary.find((f) => String(f.key).toLowerCase() === 'endpoints')?.value
  const name = summary.find((f) => String(f.key).toLowerCase() === 'name')?.value || inspect.name
  if (endpoints) return { name, endpoints, parts: splitEndpointParts(endpoints) }

  return null
}

function fieldsToEndpoint(fields) {
  const map = Object.fromEntries(fields.map((f) => [f.key, f.value]))
  const endpoints = map.Endpoints || map.endpoints || ''
  if (!endpoints) return null
  return {
    name: map.Name || map.name || '',
    endpoints,
    parts: splitEndpointParts(endpoints),
  }
}

export function splitEndpointParts(endpoints = '') {
  return String(endpoints || '')
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean)
}
