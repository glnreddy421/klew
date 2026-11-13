import {
  AbsoluteFill,
  Easing,
  Img,
  OffthreadVideo,
  Sequence,
  interpolate,
  spring,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from 'remotion'
import { FPS, theme } from './theme.js'

// Cuts of the raw take (public/take.mp4, 1920×1080). Times are seconds in the
// raw take (see src/markers.json). `cam` keyframes are in output seconds from
// the start of the cut: [t, focusX, focusY, zoom].
export const CUTS = [
  { from: 0.4, to: 6.0, rate: 1, caption: 'One home for every cluster', cam: [[0, 960, 540, 1]] },
  {
    from: 6.0, to: 13.0, rate: 1, caption: 'Cluster health at a glance',
    cam: [[0, 960, 540, 1], [2.2, 960, 540, 1], [3.6, 900, 560, 1.18], [7, 900, 560, 1.18]],
  },
  { from: 13.0, to: 22.9, rate: 1.4, caption: 'Every kind, listed and searchable', cam: [[0, 960, 540, 1]] },
  {
    from: 22.9, to: 35.3, rate: 1.15, caption: 'Compare any two revisions. See exactly what changed.',
    cam: [[0, 960, 540, 1], [4.4, 960, 540, 1], [5.6, 1560, 380, 1.55], [9.9, 1560, 380, 1.55], [10.8, 960, 540, 1]],
  },
  {
    from: 35.3, to: 47.2, rate: 1.35, caption: 'A shell, already pinned to the cluster and namespace',
    cam: [[0, 960, 540, 1], [3.4, 960, 540, 1], [4.4, 420, 740, 1.6], [7.9, 420, 740, 1.6], [8.8, 960, 540, 1]],
  },
  { from: 47.2, to: 60.2, rate: 1.8, caption: 'Pick any workloads, pods or Services…', cam: [[0, 960, 540, 1]] },
  {
    from: 60.2, to: 67.0, rate: 1, caption: '…and stream their logs live, merged in one view',
    cam: [[0, 960, 540, 1], [0.6, 960, 540, 1], [1.8, 820, 820, 1.32], [6.8, 820, 820, 1.32]],
  },
]

const TITLE = 3 * FPS
const END = 4.5 * FPS
const cutFrames = (c) => Math.round(((c.to - c.from) / c.rate) * FPS)
const FOOTAGE = CUTS.reduce((n, c) => n + cutFrames(c), 0)
export const totalFrames = TITLE + FOOTAGE + END

export const smooth = Easing.inOut(Easing.cubic)

export function camera(cam, t) {
  let k = 0
  while (k < cam.length - 1 && t > cam[k + 1][0]) k++
  const a = cam[k]
  const b = cam[Math.min(k + 1, cam.length - 1)]
  const p = b[0] === a[0] ? 1 : smooth(Math.min(1, Math.max(0, (t - a[0]) / (b[0] - a[0]))))
  const [x, y, z] = [1, 2, 3].map((i) => a[i] + (b[i] - a[i]) * p)
  // Keep the zoomed frame inside the footage.
  const tx = Math.min(0, Math.max(1920 - 1920 * z, 960 - x * z))
  const ty = Math.min(0, Math.max(1080 - 1080 * z, 540 - y * z))
  return `translate(${tx}px, ${ty}px) scale(${z})`
}

export function Caption({ text, frames, sub }) {
  const f = useCurrentFrame()
  const o = interpolate(f, [6, 16, frames - 10, frames - 2], [0, 1, 1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const y = interpolate(f, [6, 18], [14, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp', easing: Easing.out(Easing.cubic) })
  return (
    <div style={{ position: 'absolute', left: 0, right: 0, bottom: 44, display: 'flex', justifyContent: 'center', opacity: o, transform: `translateY(${y}px)` }}>
      <div
        style={{
          fontFamily: theme.sans, fontWeight: 600, fontSize: 38, letterSpacing: -0.4, color: theme.text,
          padding: '16px 30px', borderRadius: 16, background: 'rgba(12,15,20,0.82)',
          border: '1px solid rgba(255,255,255,0.10)', boxShadow: '0 18px 50px rgba(0,0,0,0.55)', backdropFilter: 'blur(14px)',
        }}
      >
        {text}
      </div>
    </div>
  )
}

export function Cut({ cut, frames }) {
  const f = useCurrentFrame()
  return (
    <AbsoluteFill>
      <AbsoluteFill style={{ transformOrigin: '0 0', transform: camera(cut.cam, f / FPS) }}>
        <OffthreadVideo src={staticFile(cut.src || 'take.mp4')} startFrom={Math.round(cut.from * FPS)} playbackRate={cut.rate} muted />
        {/* Masks hide personal details (plan name, home path) and app notices;
            boxes are in take pixels and filled with the terminal background. */}
        {(cut.masks || []).map((m, i) => (
          <div
            key={i}
            style={{ position: 'absolute', left: m.x, top: m.y, width: m.w, height: m.h, background: m.color || '#132333' }}
          />
        ))}
        {cut.highlight && (
          <div
            style={{
              position: 'absolute', left: cut.highlight.x - 8, top: cut.highlight.y - 6,
              width: cut.highlight.w + 16, height: cut.highlight.h + 12, borderRadius: 6,
              border: `2px solid ${theme.accent}`, background: 'rgba(29,143,225,0.14)',
              boxShadow: '0 0 0 4px rgba(29,143,225,0.18)',
              opacity: interpolate(f / FPS, [cut.highlight.at, cut.highlight.at + 0.35], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }),
            }}
          />
        )}
      </AbsoluteFill>
      <Caption text={cut.caption} frames={frames} />
    </AbsoluteFill>
  )
}

// Footage sits in a rounded app window on a dark backdrop.
export function Window({ children }) {
  const S = 0.92
  return (
    <AbsoluteFill
      style={{
        background: `radial-gradient(1200px 700px at 20% 0%, rgba(29,143,225,0.22), transparent 60%),
          radial-gradient(900px 600px at 100% 100%, rgba(46,204,113,0.10), transparent 60%), ${theme.bg}`,
      }}
    >
      <div
        style={{
          position: 'absolute', left: (1920 * (1 - S)) / 2, top: (1080 * (1 - S)) / 2, width: 1920 * S, height: 1080 * S,
          borderRadius: 18, overflow: 'hidden', border: '1px solid rgba(255,255,255,0.12)',
          boxShadow: '0 40px 120px rgba(0,0,0,0.65), 0 0 0 1px rgba(0,0,0,0.6)',
        }}
      >
        <div style={{ width: 1920, height: 1080, transform: `scale(${S})`, transformOrigin: '0 0', position: 'relative', overflow: 'hidden' }}>
          {children}
        </div>
      </div>
    </AbsoluteFill>
  )
}

export function Brand({ size = 120 }) {
  const f = useCurrentFrame()
  const { fps } = useVideoConfig()
  const s = spring({ frame: f, fps, config: { damping: 14, mass: 0.7 } })
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: size * 0.28, transform: `scale(${0.85 + 0.15 * s})`, opacity: s }}>
      <Img src={staticFile('klew-mark.png')} style={{ height: size, width: 'auto' }} />
      <span style={{ fontFamily: theme.display, fontWeight: 700, fontSize: size * 0.9, letterSpacing: -2, color: theme.text }}>Klew</span>
    </div>
  )
}

export function fadeInOut(f, len, inF = 10, outF = 10) {
  return interpolate(f, [0, inF, len - outF, len], [0, 1, 1, 0], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
}

function TitleCard() {
  const f = useCurrentFrame()
  const sub = interpolate(f, [14, 30], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  return (
    <AbsoluteFill style={{ background: theme.bg, alignItems: 'center', justifyContent: 'center', opacity: fadeInOut(f, TITLE, 1, 10) }}>
      <AbsoluteFill style={{ background: 'radial-gradient(900px 500px at 50% 45%, rgba(29,143,225,0.25), transparent 70%)' }} />
      <Brand />
      <div style={{ marginTop: 34, fontFamily: theme.display, fontSize: 40, color: theme.textSecondary, opacity: sub, transform: `translateY(${(1 - sub) * 10}px)` }}>
        Browse any Kubernetes cluster, live.
      </div>
    </AbsoluteFill>
  )
}

function EndCard() {
  const f = useCurrentFrame()
  const a = interpolate(f, [12, 28], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  const b = interpolate(f, [26, 42], [0, 1], { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' })
  return (
    <AbsoluteFill style={{ background: theme.bg, alignItems: 'center', justifyContent: 'center', opacity: fadeInOut(f, END, 10, 1) }}>
      <AbsoluteFill style={{ background: 'radial-gradient(900px 500px at 50% 40%, rgba(29,143,225,0.22), transparent 70%)' }} />
      <Brand size={96} />
      <div style={{ marginTop: 36, fontFamily: theme.display, fontSize: 40, fontWeight: 600, color: theme.text, opacity: a }}>
        Read-only. Klew only lists, gets and watches.
      </div>
    </AbsoluteFill>
  )
}

export function KlewResourcesDemo() {
  let at = TITLE
  return (
    <AbsoluteFill style={{ background: theme.bg }}>
      <Sequence durationInFrames={TITLE}>
        <TitleCard />
      </Sequence>
      <Sequence from={TITLE} durationInFrames={FOOTAGE}>
        <Window>
          {CUTS.map((cut) => {
            const n = cutFrames(cut)
            const from = at - TITLE
            at += n
            return (
              <Sequence key={cut.from} from={from} durationInFrames={n}>
                <Cut cut={cut} frames={n} />
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
