// Shared recorder: drives the Klew UI in Chromium and captures crisp frames via
// the DevTools screencast, with a drawn cursor and click rings.
import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

export const W = 1920
export const H = 1080

const CHROME =
  process.env.CHROME ||
  path.join(
    os.homedir(),
    'Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
  )

const CURSOR_SCRIPT = `
(() => {
  const install = () => {
    if (document.getElementById('__demo_cursor')) return
    const style = document.createElement('style')
    style.textContent = \`
      #__demo_cursor{position:fixed;left:0;top:0;width:26px;height:26px;z-index:2147483647;pointer-events:none;
        transform:translate(-100px,-100px);filter:drop-shadow(0 2px 3px rgba(0,0,0,.55))}
      .__demo_ring{position:fixed;width:44px;height:44px;margin:-22px 0 0 -22px;border-radius:50%;
        border:2px solid rgba(29,143,225,.95);background:rgba(29,143,225,.18);z-index:2147483646;pointer-events:none;
        animation:__demo_ring .55s ease-out forwards}
      @keyframes __demo_ring{from{transform:scale(.35);opacity:1}to{transform:scale(1.25);opacity:0}}
    \`
    document.head.appendChild(style)
    const c = document.createElement('div')
    c.id = '__demo_cursor'
    c.innerHTML = '<svg viewBox="0 0 24 24" width="26" height="26"><path d="M4 2.5l15.5 9.2-6.9 1.4 4.1 7.3-3 1.7-4.1-7.4L4.6 19z" fill="#fff" stroke="#111" stroke-width="1.3" stroke-linejoin="round"/></svg>'
    document.body.appendChild(c)
    addEventListener('mousemove', e => { c.style.transform = 'translate(' + (e.clientX - 4) + 'px,' + (e.clientY - 2) + 'px)' }, true)
    addEventListener('mousedown', e => {
      const r = document.createElement('div'); r.className = '__demo_ring'
      r.style.left = e.clientX + 'px'; r.style.top = e.clientY + 'px'
      document.body.appendChild(r); setTimeout(() => r.remove(), 700)
    }, true)
  }
  if (document.readyState === 'loading') addEventListener('DOMContentLoaded', install); else install()
})()
`

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const ease = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2)

export async function createRecorder({ out, url = 'http://localhost:34115', dsf = 2, width = W, height = H }) {
  const OUT = path.resolve(out)
  const FRAMES = path.join(OUT, 'frames')
  fs.rmSync(OUT, { recursive: true, force: true })
  fs.mkdirSync(FRAMES, { recursive: true })

  const browser = await chromium.launch({
    headless: !process.env.HEADED,
    executablePath: fs.existsSync(CHROME) ? CHROME : undefined,
  })
  const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: dsf, colorScheme: 'dark' })
  await context.addInitScript(CURSOR_SCRIPT)
  await context.addInitScript(() => {
    try { if (!localStorage.getItem('klew-desktop-theme')) localStorage.setItem('klew-desktop-theme', 'dark') } catch {}
  })
  const page = await context.newPage()
  page.on('pageerror', (e) => { if (!/reading 'nodes'/.test(e.message)) console.error('[pageerror]', e.message) })

  let mouse = { x: width / 2, y: height / 2 }
  const cdp = await context.newCDPSession(page)
  const frames = []
  let t0 = null
  cdp.on('Page.screencastFrame', async ({ data, metadata, sessionId }) => {
    if (t0 === null) t0 = metadata.timestamp
    const file = `${String(frames.length).padStart(6, '0')}.jpg`
    fs.writeFileSync(path.join(FRAMES, file), Buffer.from(data, 'base64'))
    frames.push({ file, t: metadata.timestamp - t0 })
    cdp.send('Page.screencastFrameAck', { sessionId }).catch(() => {})
  })

  let recStart = 0
  const markers = []
  const now = () => Date.now() / 1000 - recStart

  const r = {
    page,
    async open() {
      await page.goto(url, { waitUntil: 'domcontentloaded' })
    },
    async start() {
      await page.mouse.move(mouse.x, mouse.y)
      await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: width * dsf, maxHeight: height * dsf, everyNthFrame: 1 })
      recStart = Date.now() / 1000
      await sleep(300)
    },
    async mark(name, locator) {
      const m = { name, t: now() }
      if (locator) {
        const box = await locator.boundingBox().catch(() => null)
        if (box) m.box = box
      }
      markers.push(m)
      console.log(`[${m.t.toFixed(2)}s] ${name}`)
    },
    async glide(x, y, ms = 650) {
      const from = { ...mouse }
      const steps = Math.max(8, Math.round(ms / 16))
      for (let i = 1; i <= steps; i++) {
        const k = ease(i / steps)
        await page.mouse.move(from.x + (x - from.x) * k, from.y + (y - from.y) * k)
        await sleep(ms / steps)
      }
      mouse = { x, y }
    },
    async hover(locator, ms) {
      await locator.waitFor({ state: 'visible', timeout: 30000 })
      const b = await locator.boundingBox()
      await r.glide(b.x + b.width / 2, b.y + b.height / 2, ms)
      return b
    },
    async click(locator, { ms = 650, pause = 220 } = {}) {
      await r.hover(locator, ms)
      await sleep(pause)
      await page.mouse.down()
      await sleep(70)
      await page.mouse.up()
    },
    async type(text, delay = 80) {
      for (const ch of text) {
        await page.keyboard.type(ch)
        await sleep(delay + Math.random() * 40)
      }
    },
    async wheel(dy, steps = 12, x = mouse.x, y = mouse.y) {
      await r.glide(x, y, 300)
      for (let i = 0; i < steps; i++) {
        await page.mouse.wheel(0, dy / steps)
        await sleep(30)
      }
    },
    /** Text of the visible terminal (xterm DOM renderer). */
    async terminalText() {
      return page.evaluate(() => {
        const all = [...document.querySelectorAll('.xterm')].filter((x) => x.offsetParent)
        const t = all.at(-1)
        return t ? [...t.querySelectorAll('.xterm-rows > div')].map((d) => d.textContent).join('\n') : ''
      })
    },
    async waitTerminal(pred, timeout = 60000, every = 250) {
      const end = Date.now() + timeout
      while (Date.now() < end) {
        const txt = await r.terminalText()
        if (pred(txt)) return txt
        await sleep(every)
      }
      throw new Error('terminal wait timed out')
    },
    async finish() {
      await cdp.send('Page.stopScreencast')
      await sleep(300)
      const shift = recStart - (t0 !== null ? t0 : recStart)
      for (const m of markers) m.t = +(m.t + shift).toFixed(3)
      fs.writeFileSync(path.join(OUT, 'frames.json'), JSON.stringify({ dsf, width, height, frames }, null, 1))
      fs.writeFileSync(path.join(OUT, 'markers.json'), JSON.stringify(markers, null, 2))
      console.log(`frames: ${frames.length}, duration: ${frames.at(-1)?.t.toFixed(1)}s`)
      await browser.close()
    },
  }
  return r
}
