function plainText(value) {
  // eslint-disable-next-line no-control-regex -- notices quote kubeconfig errors; keep escapes out of xterm
  return String(value || '').replace(/[\u0000-\u001f\u007f]/g, '')
}

/** Yellow one-liner when the shell could not be pinned to a cluster; empty otherwise. */
export function terminalNotice(info) {
  if (!info?.notice) return ''
  return `\x1b[33m${plainText(info.notice)}\x1b[0m\r\n`
}
