import { TerminalPanel } from '../components/TerminalPanel'

/** Full-workspace terminal surface — lives on the Terminal activity tab only. */
export function TerminalView({
  cluster,
  shellPref,
  appearance,
  onChangeShell,
  shellRestartToken = 0,
  hidden = false,
}) {
  return (
    <div
      className={`terminal-workspace ${hidden ? 'terminal-workspace-hidden' : ''}`}
      aria-hidden={hidden}
    >
      <TerminalPanel
        layout="workspace"
        open
        cluster={cluster}
        shellPref={shellPref}
        appearance={appearance}
        onChangeShell={onChangeShell}
        shellRestartToken={shellRestartToken}
      />
    </div>
  )
}
