// Turns the variable-rate screencast frames into a constant 30fps video.
//   node assemble.mjs [rawDir] [outMp4] [markersOut]
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

const [raw = 'out/raw', outMp4 = 'public/take.mp4', markersOut = 'src/markers.json'] = process.argv.slice(2)
const { frames } = JSON.parse(fs.readFileSync(path.join(raw, 'frames.json'), 'utf8'))
const lines = []
for (let i = 0; i < frames.length; i++) {
  const next = frames[i + 1]?.t ?? frames[i].t + 0.5
  lines.push(`file 'frames/${frames[i].file}'`, `duration ${Math.max(0.001, next - frames[i].t).toFixed(4)}`)
}
lines.push(`file 'frames/${frames.at(-1).file}'`)
fs.writeFileSync(path.join(raw, 'concat.txt'), lines.join('\n'))
fs.mkdirSync(path.dirname(outMp4), { recursive: true })
execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', path.join(raw, 'concat.txt'),
  '-vf', 'fps=30,scale=1920:1080:flags=lanczos,format=yuv420p', '-c:v', 'libx264', '-preset', 'slow', '-crf', '14', outMp4], { stdio: 'inherit' })
fs.copyFileSync(path.join(raw, 'markers.json'), markersOut)
console.log(`wrote ${outMp4}`)
