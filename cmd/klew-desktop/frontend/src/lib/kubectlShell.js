/** Shell-safe quoting — matches backend kubectl_manifest.go behavior. */

export function shellQuote(value) {
  const s = String(value ?? '')
  if (!s) return "''"
  if (/^[A-Za-z0-9._/-]+$/.test(s)) return s
  return `'${s.replace(/'/g, `'\\''`)}'`
}

export function commandLine(parts) {
  return `${parts.filter(Boolean).join(' ')}\n`
}

/** Only emit --kubeconfig when the path is non-default (custom kubeconfig). */
export function kubeconfigForCommandFlag(path) {
  const trimmed = String(path || '').trim()
  if (!trimmed) return ''
  if (trimmed === '~/.kube/config' || trimmed.endsWith('/.kube/config')) return ''
  return trimmed
}
