import { AbsoluteFill, Sequence, interpolate, useCurrentFrame } from 'remotion'
import { Brand, Cut, Window, fadeInOut } from './Video.jsx'
import { FPS, theme } from './theme.js'
import { MCP_CUTS } from './mcpCuts.js'

// Cuts from take2.mp4 (desktop app + MCP) and take3.mp4 (CLI). Times are
// seconds in the raw take (src/markers2.json, src/markers3.json); cam keyframes
// are output seconds from the start of the cut: [t, focusX, focusY, zoom].
const T2 = 'take2.mp4'
const T3 = 'take3.mp4'
const CUTS_BASE = [
  // Desktop app
  { src: T2, section: 'Desktop app', from: 0.3, to: 10.9, rate: 1.3, caption: 'Pick a namespace. Click Investigate.',
    cam: [[0, 960, 540, 1], [2.6, 960, 540, 1], [3.6, 640, 200, 1.6], [7.4, 640, 200, 1.6], [8.1, 960, 540, 1]] },
  { src: T2, section: 'Desktop app', from: 10.9, to: 21.9, rate: 1.15, caption: 'A verdict, not a dump',
    cam: [[0, 960, 540, 1], [1.4, 960, 540, 1], [2.6, 640, 330, 1.55], [8.6, 640, 330, 1.55], [9.5, 960, 540, 1]] },
  { src: T2, section: 'Desktop app', from: 21.9, to: 28.1, rate: 1.1, caption: 'Every failing pod, with its containers and events',
    cam: [[0, 960, 540, 1]] },
  { src: T2, section: 'Desktop app', from: 28.1, to: 34.3, rate: 1.1, caption: 'The evidence behind each claim',
    cam: [[0, 960, 540, 1]] },
  { src: T2, section: 'Local history', from: 34.3, to: 47.3, rate: 1.2, caption: 'Kubernetes keeps Events ~1 hour. Klew records them in a local database.',
    cam: [[0, 960, 540, 1], [4.6, 960, 540, 1], [5.8, 1080, 430, 1.55], [10.8, 1080, 430, 1.55]] },
  // CLI
  { src: T3, section: 'CLI', from: 0.3, to: 11.9, rate: 1.35, caption: 'Same engine in your terminal: klew investigate',
    cam: [[0, 960, 540, 1], [0.8, 960, 540, 1], [2.0, 1700, 160, 1.7], [4.6, 1700, 160, 1.7], [5.6, 960, 540, 1], [6.6, 420, 140, 1.8], [8.6, 420, 140, 1.8]] },
  { src: T3, section: 'CLI', from: 11.9, to: 24.0, rate: 3, caption: 'It watches the namespace for 10 seconds, then reports',
    cam: [[0, 420, 160, 1.8], [4, 420, 160, 1.8]] },
  { src: T3, section: 'CLI', from: 25.6, to: 28.6, rate: 1, caption: 'The same verdict as text, or JSON for scripts and CI',
    cam: [[0, 600, 170, 1.75], [3, 600, 170, 1.75]] },
  // Holds on the report before it scrolls (cli-scroll at 31.85s).
  { src: T3, section: 'CLI', from: 28.6, to: 31.8, rate: 0.8, caption: 'It reads the same local history, older than the API keeps',
    cam: [[0, 600, 170, 1.75], [0.9, 560, 900, 1.5], [4, 560, 900, 1.5]],
    // "Includes 6 events from the desktop app's history that the API no longer keeps."
    highlight: { x: 12, y: 970, w: 590, h: 12, at: 1.1 } },
  // MCP
  { src: T2, section: 'MCP', from: 86.2, to: 96.8, rate: 1.4, caption: 'Give it to your AI assistant: klew mcp',
    cam: [[0, 520, 140, 1.9], [7.6, 520, 140, 1.9]] },
  { src: T2, section: 'MCP', from: 99.6, to: 106.6, rate: 1.4, caption: 'Ask in plain words',
    cam: [[0, 960, 540, 1], [1.0, 700, 1000, 1.6], [5, 700, 1000, 1.6]] },
  { src: T2, section: 'MCP', from: 106.6, to: 141.4, rate: 6, caption: 'Claude calls Klew’s read-only tools',
    cam: [[0, 520, 230, 1.8], [5.8, 520, 230, 1.8]] },
  { src: T2, section: 'MCP', from: 141.4, to: 152.4, rate: 1, caption: 'Every broken workload, with its cause',
    cam: [[0, 960, 540, 1], [0.8, 960, 540, 1], [2.0, 900, 470, 1.15], [11, 900, 470, 1.15]] },
]

// MCP part: the cleaned take shared with the combined video.
const CUTS = [...CUTS_BASE.filter((c) => c.section !== 'MCP'), ...MCP_CUTS.map((c) => ({ ...c, section: 'MCP' }))]

const TITLE = 3 * FPS
const END = 5.5 * FPS
const cutFrames = (c) => Math.round(((c.to - c.from) / c.rate) * FPS)
const FOOTAGE = CUTS.reduce((n, c) => n + cutFrames(c), 0)
export const totalFrames2 = TITLE + FOOTAGE + END

function SectionChip({ label, frames, show }) {
  const f = useCurrentFrame()
  const o = show ? interpolate(f, [0, 10, frames - 6, frames], [0, 1, 1, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }) : 1
  return (
    <div
      style={{
        position: 'absolute', top: 70, left: 34, opacity: o, fontFamily: theme.sans, fontSize: 26, fontWeight: 600,
        color: theme.text, padding: '10px 20px', borderRadius: 999, background: 'rgba(29,143,225,0.92)',
        boxShadow: '0 10px 30px rgba(0,0,0,0.45)', letterSpacing: 0.2,
      }}
    >
      {label}
    </div>
  )
}

function TitleCard() {
  const f = useCurrentFrame()
  const sub = interpolate(f, [14, 30], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  return (
    <AbsoluteFill style={{ background: theme.bg, alignItems: 'center', justifyContent: 'center', opacity: fadeInOut(f, TITLE, 1, 10) }}>
      <AbsoluteFill style={{ background: 'radial-gradient(900px 500px at 50% 45%, rgba(29,143,225,0.25), transparent 70%)' }} />
      <Brand />
      <div style={{ marginTop: 34, fontFamily: theme.display, fontSize: 40, color: theme.textSecondary, opacity: sub }}>
        Investigate: one read-only engine. Desktop, CLI and MCP.
      </div>
    </AbsoluteFill>
  )
}

function EndCard() {
  const f = useCurrentFrame()
  const a = interpolate(f, [12, 28], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const b = interpolate(f, [26, 42], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const line = (prompt, text) => (
    <div style={{ fontFamily: theme.mono, fontSize: 28, color: theme.text, padding: '12px 24px', borderRadius: 12, background: theme.surface, border: `1px solid ${theme.border}`, minWidth: 900 }}>
      <span style={{ color: theme.accent }}>{prompt}</span> {text}
    </div>
  )
  return (
    <AbsoluteFill style={{ background: theme.bg, alignItems: 'center', justifyContent: 'center', opacity: fadeInOut(f, END, 10, 1) }}>
      <AbsoluteFill style={{ background: 'radial-gradient(900px 500px at 50% 40%, rgba(29,143,225,0.22), transparent 70%)' }} />
      <Brand size={90} />
      <div style={{ marginTop: 30, fontFamily: theme.display, fontSize: 38, fontWeight: 600, color: theme.text, opacity: a, textAlign: 'center' }}>
        Read-only. Your event history stays on your machine.
      </div>
    </AbsoluteFill>
  )
}

export function KlewInvestigateDemo() {
  let at = 0
  return (
    <AbsoluteFill style={{ background: theme.bg }}>
      <Sequence durationInFrames={TITLE}>
        <TitleCard />
      </Sequence>
      <Sequence from={TITLE} durationInFrames={FOOTAGE}>
        <Window>
          {CUTS.map((cut, i) => {
            const n = cutFrames(cut)
            const from = at
            at += n
            const firstOfSection = i === 0 || CUTS[i - 1].section !== cut.section
            return (
              <Sequence key={`${cut.src}-${cut.from}`} from={from} durationInFrames={n}>
                <Cut cut={cut} frames={n} />
                <SectionChip label={cut.section} frames={n} show={firstOfSection} />
              </Sequence>
            )
          })}
        </Window>
      </Sequence>
      <Sequence from={TITLE + FOOTAGE} durationInFrames={END}>
        <EndCard />
      </Sequence>
    </AbsoluteFill>
  )
}

export { CUTS as CUTS2 }
