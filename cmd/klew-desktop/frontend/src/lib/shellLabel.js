/** Short shell name from backend path, e.g. /bin/zsh → zsh */
export function shellLabel(shellPath) {
  if (!shellPath) return ''
  const base = String(shellPath).split(/[/\\]/).pop() || shellPath
  return base.replace(/\.exe$/i, '')
}
