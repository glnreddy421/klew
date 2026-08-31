/** Decode a Kubernetes secret data value (base64) for display. */
export function decodeSecretValue(encoded) {
  if (!encoded) return '—'
  try {
    const binary = atob(encoded)
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  } catch {
    try {
      const binary = atob(encoded)
      const hex = [...binary]
        .map((c) => c.charCodeAt(0).toString(16).padStart(2, '0'))
        .join(' ')
      if (hex.length > 96) return `0x ${hex.slice(0, 96)}… (${binary.length} bytes)`
      return `0x ${hex}`
    } catch {
      return '<invalid base64>'
    }
  }
}
