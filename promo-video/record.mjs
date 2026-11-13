// Drives the real Klew UI (wails dev at localhost:34115) through the demo flow
// in SCRIPT.md and records it as JPEG frames via the Chrome DevTools screencast.
// Output: out/raw/frames/*.jpg, out/raw/frames.json, out/raw/markers.json
//
//   node record.mjs            # headless
//   HEADED=1 node record.mjs   # watch it run
import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

const APP = process.env.APP_URL || 'http://localhost:34115'
const OUT = path.resolve('out/raw')
const FRAMES = path.join(OUT, 'frames')
const W = 1920
const H = 1080
const DSF = Number(process.env.DSF || 2)
const CHROME =
  process.env.CHROME ||
  path.join(
    os.homedir(),
    'Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
  )

fs.rmSync(OUT, { recursive: true, force: true })
fs.mkdirSync(FRAMES, { recursive: true })

// A visible cursor: headless Chrome draws none, so the page draws one that
// follows real mouse events, plus a ring on each click.
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

const browser = await chromium.launch({
  headless: !process.env.HEADED,
  executablePath: fs.existsSync(CHROME) ? CHROME : undefined,
})
const context = await browser.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: DSF, colorScheme: 'dark' })
await context.addInitScript(CURSOR_SCRIPT)
// Same look as the user's app: dark theme (a fresh profile defaults to light).
await context.addInitScript(() => {
  try { if (!localStorage.getItem('klew-desktop-theme')) localStorage.setItem('klew-desktop-theme', 'dark') } catch {}
})
const page = await context.newPage()
page.on('pageerror', (e) => console.error('[pageerror]', e.message))

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
let mouse = { x: W / 2, y: H / 2 }

// Screencast ---------------------------------------------------------------
const cdp = await context.newCDPSession(page)
const frames = []
let t0 = null
cdp.on('Page.screencastFrame', async ({ data, metadata, sessionId }) => {
  const ts = metadata.timestamp
  if (t0 === null) t0 = ts
  const file = `${String(frames.length).padStart(6, '0')}.jpg`
  fs.writeFileSync(path.join(FRAMES, file), Buffer.from(data, 'base64'))
  frames.push({ file, t: ts - t0 })
  cdp.send('Page.screencastFrameAck', { sessionId }).catch(() => {})
})
const wallStart = () => Date.now() / 1000
let recStartWall = 0
const markers = []
const now = () => wallStart() - recStartWall

async function mark(name, locator) {
  const m = { name, t: now() }
  if (locator) {
    const box = await locator.boundingBox().catch(() => null)
    if (box) m.box = box
  }
  markers.push(m)
  console.log(`[${m.t.toFixed(2)}s] ${name}`)
}

// Motion helpers --------------------------------------------------------------
const ease = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2)

async function glide(x, y, ms = 650) {
  const from = { ...mouse }
  const steps = Math.max(8, Math.round(ms / 16))
  for (let i = 1; i <= steps; i++) {
    const k = ease(i / steps)
    await page.mouse.move(from.x + (x - from.x) * k, from.y + (y - from.y) * k)
    await sleep(ms / steps)
  }
  mouse = { x, y }
}

async function center(locator) {
  await locator.waitFor({ state: 'visible', timeout: 20000 })
  const b = await locator.boundingBox()
  return { x: b.x + b.width / 2, y: b.y + b.height / 2, box: b }
}

async function hover(locator, ms) {
  const c = await center(locator)
  await glide(c.x, c.y, ms)
  return c
}

async function click(locator, { ms = 650, pause = 220 } = {}) {
  await hover(locator, ms)
  await sleep(pause)
  await page.mouse.down()
  await sleep(70)
  await page.mouse.up()
}

async function typeText(text, delay = 85) {
  for (const ch of text) {
    await page.keyboard.type(ch)
    await sleep(delay + Math.random() * 40)
  }
}

async function wheel(dy, steps = 12, x = mouse.x, y = mouse.y) {
  await glide(x, y, 300)
  for (let i = 0; i < steps; i++) {
    await page.mouse.wheel(0, dy / steps)
    await sleep(28)
  }
}

// Flow ------------------------------------------------------------------------
await page.goto(APP, { waitUntil: 'domcontentloaded' })
const openDocker = page.getByTitle('Open docker-desktop')
await openDocker.waitFor({ timeout: 60000 })
await sleep(2500) // let tiles settle (status, versions)
await page.mouse.move(mouse.x, mouse.y)

await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: W * DSF, maxHeight: H * DSF, everyNthFrame: 1 })
recStartWall = wallStart()
await sleep(400)

// 2. Home
await mark('home', page.locator('main, #root').first())
await sleep(1200)
await hover(page.getByTitle('Open kind-klew-a'), 900)
await sleep(500)
await hover(page.getByTitle('Open kind-klew-b'), 700)
await sleep(500)
await mark('open-docker', openDocker)
await click(openDocker, { ms: 900 })

// 3. Overview
const overviewTitle = page.getByRole('heading', { name: 'Overview' }).first()
await overviewTitle.waitFor({ timeout: 30000 }).catch(() => {})
await sleep(1500)
const workloadsHeading = page.getByText('Workloads', { exact: true }).nth(1)
await mark('overview', workloadsHeading)
await hover(workloadsHeading, 900)
await sleep(3800)

// 4. Pods, then Deployments + search
const podsNav = page.getByRole('button', { name: /^Pods(\s+\d+)?$/ })
await mark('pods', podsNav)
await click(podsNav)
await sleep(2600)
const deployNav = page.getByRole('button', { name: /^Deployments(\s+\d+)?$/ })
await mark('deployments', deployNav)
await click(deployNav)
await sleep(1600)
const search = page.getByPlaceholder('Search deployments…')
await click(search)
await mark('search', search)
await typeText('payment')
await sleep(1400)

// 5. Inspector + Compare
const row = page.getByRole('row').filter({ hasText: 'klew-lab' }).filter({ hasText: /payment-api/ }).first()
const rowName = row.getByText('payment-api', { exact: true }).first()
await mark('select-payment-api', rowName)
await click(rowName)
await sleep(2200)
const compareTab = page.getByRole('tab', { name: /^Compare/ })
await mark('compare-tab', compareTab)
await click(compareTab)
await page.locator('.compare-diff').first().waitFor({ timeout: 20000 }).catch(() => {})
await sleep(600)
await mark('compare-diff', page.locator('.compare-diff').first())
await hover(page.locator('.compare-diff').first(), 900)
await sleep(4200)

// Clear search so the full list is back for log selection.
await click(search, { ms: 700 })
await page.keyboard.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A')
await page.keyboard.press('Backspace')
await sleep(900)

// 6. Terminal
const showTerminal = page.getByRole('button', { name: 'Show terminal' })
await mark('show-terminal', showTerminal)
await click(showTerminal)
const xterm = page.locator('.xterm').first()
await xterm.waitFor({ timeout: 20000 })
await sleep(1800)
await click(xterm, { ms: 700 })
await mark('terminal', xterm)
await sleep(400)
await typeText('kubectl get deploy -n klew-lab', 70)
await sleep(400)
await page.keyboard.press('Enter')
await sleep(3800)

// 7. Multi-stream logs
const isVisibleAboveDock = (loc) =>
  loc.evaluate((el) => {
    const r = el.getBoundingClientRect()
    const dock = document.querySelector('[aria-label="Open terminals and log streams"]')
    const limit = dock ? dock.getBoundingClientRect().top - 20 : innerHeight
    return r.top > 150 && r.bottom < limit
  })
const pick = (name) => page.getByRole('checkbox', { name: `Include Deployment/${name} in live log stream` })
await mark('pick-logs')
for (const name of ['audit-log', 'notifier', 'payment-api']) {
  const cb = pick(name)
  for (let tries = 0; tries < 6 && !(await isVisibleAboveDock(cb)); tries++) {
    const tb = await page.locator('.entity-table-scroll').first().boundingBox()
    await wheel(150, 10, tb.x + tb.width * 0.45, tb.y + 220)
    await sleep(350)
  }
  await click(cb, { ms: 600, pause: 150 })
  await sleep(450)
}
await sleep(400)
// The "N selected · Stream logs" bar sits at the top of the scrolled table.
const scroller = page.locator('.entity-table-scroll').first()
const sb = await scroller.boundingBox()
await glide(sb.x + sb.width * 0.45, sb.y + 220, 400)
while ((await scroller.evaluate((el) => el.scrollTop)) > 0) {
  await page.mouse.wheel(0, -60)
  await sleep(28)
}
await sleep(500)
const streamBtn = page.locator('.entity-table-stream-logs-btn').first()
await mark('stream-button', streamBtn)
await click(streamBtn)
await sleep(1500)
const dock = page.locator('[aria-label="Open terminals and log streams"]').first()
await mark('logs', dock)
await glide(W * 0.55, H * 0.82, 900)
await sleep(7000)
await mark('end')
await sleep(600)

await cdp.send('Page.stopScreencast')
await sleep(300)
fs.writeFileSync(path.join(OUT, 'frames.json'), JSON.stringify({ dsf: DSF, width: W, height: H, frames }, null, 1))
// Screencast timestamps are epoch seconds, so put markers on the same clock.
const shift = recStartWall - (t0 ?? recStartWall)
for (const m of markers) m.t = +(m.t + shift).toFixed(3)
fs.writeFileSync(path.join(OUT, 'markers.json'), JSON.stringify(markers, null, 2))
console.log(`frames: ${frames.length}, duration: ${frames.at(-1)?.t.toFixed(1)}s`)
await browser.close()
