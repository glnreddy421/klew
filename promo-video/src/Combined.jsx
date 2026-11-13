import { AbsoluteFill, Easing, Sequence, interpolate, useCurrentFrame } from 'remotion'
import { Brand, CUTS as CUTS1, Cut, fadeInOut } from './Video.jsx'
import { CUTS2 } from './Video2.jsx'
import { MCP_CUTS } from './mcpCuts.js'
import { FPS, theme } from './theme.js'

// One video from both demos. Every cut belongs to a chapter; the chapter bar
// under the window shows where the viewer is.
const CHAPTERS = ['Resources', 'Live logs', 'Investigate', 'CLI', 'MCP']

// Video 1: Home → terminal is Resources; picking pods and streaming is Live logs.
const V1 = CUTS1.map((c, i) => ({ ...c, src: c.src || 'take.mp4', chapter: i < 5 ? 'Resources' : 'Live logs' }))
const V2 = CUTS2.map((c) => ({
  ...c,
  chapter: c.section === 'Desktop app' || c.section === 'Local history' ? 'Investigate' : c.section,
}))
const MCP = MCP_CUTS.map((c) => ({ ...c, chapter: 'MCP' }))

const CUTS = [...V1, ...V2.filter((c) => c.chapter !== 'MCP'), ...MCP]

const TITLE = 3 * FPS
const END = 6 * FPS
const cutFrames = (c) => Math.round(((c.to - c.from) / c.rate) * FPS)
const FOOTAGE = CUTS.reduce((n, c) => n + cutFrames(c), 0)
export const totalFramesCombined = TITLE + FOOTAGE + END

// Frame ranges (within the footage) of each chapter.
const RANGES = (() => {
  const r = {}
  let at = 0
  for (const c of CUTS) {
    const n = cutFrames(c)
    r[c.chapter] ||= { start: at, end: at }
    r[c.chapter].end = at + n
    at += n
  }
  return r
})()

// Window geometry: smaller than the single videos to leave room for the bar.
const S = 0.86
const WIN = { w: 1920 * S, h: 1080 * S, left: (1920 * (1 - S)) / 2, top: 24 }

function ChapterBar() {
  const f = useCurrentFrame()
  const enter = interpolate(f, [0, 14], [0, 1], { extrapolateRight: 'clamp', easing: Easing.out(Easing.cubic) })
  const gap = 14
  const segW = (WIN.w - gap * (CHAPTERS.length - 1)) / CHAPTERS.length
  return (
    <div
      style={{
        position: 'absolute', left: WIN.left, top: WIN.top + WIN.h + 26, width: WIN.w, display: 'flex', gap,
        opacity: enter, transform: `translateY(${(1 - enter) * 12}px)`,
      }}
    >
      {CHAPTERS.map((name, i) => {
        const { start, end } = RANGES[name]
        const p = Math.min(1, Math.max(0, (f - start) / (end - start)))
        const active = f >= start && f < end
        const done = f >= end
        return (
          <div key={name} style={{ width: segW }}>
            <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, marginBottom: 9 }}>
              <span style={{ fontFamily: theme.mono, fontSize: 18, color: active ? theme.accent : theme.textSecondary, opacity: active || done ? 1 : 0.6 }}>
                {String(i + 1).padStart(2, '0')}
              </span>
              <span
                style={{
                  fontFamily: theme.sans, fontSize: 25, fontWeight: active ? 700 : 500, letterSpacing: -0.2,
                  color: active ? theme.text : theme.textSecondary, opacity: active || done ? 1 : 0.6,
                }}
              >
                {name}
              </span>
            </div>
            <div style={{ height: 6, borderRadius: 3, background: 'rgba(255,255,255,0.10)', overflow: 'hidden' }}>
              <div
                style={{
                  width: `${p * 100}%`, height: '100%', borderRadius: 3,
                  background: done ? 'rgba(29,143,225,0.55)' : theme.accent,
                  boxShadow: active ? '0 0 12px rgba(29,143,225,0.7)' : 'none',
                }}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}

function Stage({ children, bare }) {
  // Bare: footage edge to edge, for pages that draw their own window and chapters.
  if (bare) return <AbsoluteFill style={{ background: theme.bg }}>{children}</AbsoluteFill>
  return (
    <AbsoluteFill
      style={{
        background: `radial-gradient(1200px 700px at 20% 0%, rgba(29,143,225,0.20), transparent 60%),
          radial-gradient(900px 600px at 100% 100%, rgba(46,204,113,0.08), transparent 60%), ${theme.bg}`,
      }}
    >
      <div
        style={{
          position: 'absolute', left: WIN.left, top: WIN.top, width: WIN.w, height: WIN.h, borderRadius: 16,
          overflow: 'hidden', border: '1px solid rgba(255,255,255,0.12)',
          boxShadow: '0 40px 120px rgba(0,0,0,0.65), 0 0 0 1px rgba(0,0,0,0.6)',
        }}
      >
        <div style={{ width: 1920, height: 1080, transform: `scale(${S})`, transformOrigin: '0 0', position: 'relative', overflow: 'hidden' }}>
          {children}
        </div>
      </div>
      <ChapterBar />
    </AbsoluteFill>
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
        Browse, stream and investigate Kubernetes. Read-only.
      </div>
    </AbsoluteFill>
  )
}

function EndCard() {
  const f = useCurrentFrame()
  const a = interpolate(f, [12, 28], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const b = interpolate(f, [26, 42], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const chip = (text) => (
    <div
      style={{
        fontFamily: theme.display, fontSize: 28, fontWeight: 600, color: theme.text, padding: '12px 28px',
        borderRadius: 999, background: theme.surface, border: `1px solid ${theme.border}`,
      }}
    >
      {text}
    </div>
  )
  return (
    <AbsoluteFill style={{ background: theme.bg, alignItems: 'center', justifyContent: 'center', opacity: fadeInOut(f, END, 10, 1) }}>
      <AbsoluteFill style={{ background: 'radial-gradient(900px 500px at 50% 40%, rgba(29,143,225,0.22), transparent 70%)' }} />
      <Brand size={90} />
      <div style={{ marginTop: 30, fontFamily: theme.display, fontSize: 38, fontWeight: 600, color: theme.text, opacity: a, textAlign: 'center' }}>
        Read-only. Your event history stays on your machine.
      </div>
      <div style={{ marginTop: 40, display: 'flex', gap: 18, opacity: b }}>
        {chip('Desktop app')}
        {chip('CLI')}
        {chip('MCP server')}
      </div>
    </AbsoluteFill>
  )
}

export function KlewCombinedDemo({ bare = false }) {
  let at = 0
  return (
    <AbsoluteFill style={{ background: theme.bg }}>
      <Sequence durationInFrames={TITLE}>
        <TitleCard />
      </Sequence>
      <Sequence from={TITLE} durationInFrames={FOOTAGE}>
        <Stage bare={bare}>
          {CUTS.map((cut) => {
            const n = cutFrames(cut)
            const from = at
            at += n
            return (
              <Sequence key={`${cut.src}-${cut.from}`} from={from} durationInFrames={n}>
                <Cut cut={cut} frames={n} />
              </Sequence>
            )
          })}
        </Stage>
      </Sequence>
      <Sequence from={TITLE + FOOTAGE} durationInFrames={END}>
        <EndCard />
      </Sequence>
    </AbsoluteFill>
  )
}
