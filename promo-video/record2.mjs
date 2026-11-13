// Video 2 (SCRIPT-2.md): Investigate in the desktop app, the CLI and MCP.
//   node record2.mjs && node assemble.mjs out/raw2 public/take2.mp4 src/markers2.json
import { createRecorder, sleep, W, H } from './lib/recorder.mjs'

const NS = 'holdout'
const QUESTION = 'Why is the holdout namespace on docker-desktop failing? Use klew.'

const r = await createRecorder({ out: 'out/raw2' })
const { page } = r

await r.open()
const tile = page.getByTitle('Open docker-desktop')
await tile.waitFor({ timeout: 60000 })
await sleep(2500)
await r.start()

// --- Desktop app ---------------------------------------------------------------
await r.mark('home')
await sleep(900)
await r.click(tile, { ms: 900 })
await page.getByRole('heading', { name: 'Overview' }).first().waitFor({ timeout: 30000 }).catch(() => {})
await sleep(1200)

const modeBtn = page.getByTitle('Investigate one namespace or object (⌘K)')
await r.mark('investigate-mode', modeBtn)
await r.click(modeBtn)
await sleep(900)
const query = page.getByRole('group', { name: 'What to investigate' })
const scopeBtn = query.getByRole('button', { name: /^Scope:/ })
await r.mark('scope', scopeBtn)
await r.click(scopeBtn)
await sleep(500)
await page.getByPlaceholder('Search or type namespace…').waitFor({ timeout: 10000 })
await r.type('hold', 110)
await sleep(400)
const opt = page.getByRole('option', { name: new RegExp(NS) }).first()
await r.click(opt, { ms: 500 })
await sleep(600)
const run = page.locator('.investigate-bar-run')
await r.mark('run', run)
await r.click(run)

const tabs = page.getByRole('navigation', { name: 'Investigation results' })
await tabs.waitFor({ timeout: 30000 })
await sleep(4500)
await r.mark('overview', page.locator('main').first())
await r.glide(W * 0.35, H * 0.3, 900)
await sleep(4500)

const failuresTab = tabs.getByRole('button', { name: /^Failures/ })
await r.mark('failures', failuresTab)
await r.click(failuresTab)
await sleep(5000)

const evidenceTab = tabs.getByRole('button', { name: /^Evidence/ })
await r.mark('evidence', evidenceTab)
await r.click(evidenceTab)
await sleep(5000)

// --- Local history ---------------------------------------------------------------
const settingsBtn = page.getByRole('button', { name: 'Settings', exact: true })
await r.mark('settings', settingsBtn)
await r.click(settingsBtn)
await sleep(900)
const invSection = page.locator('.settings-nav-item', { hasText: 'Investigation' }).first()
await r.click(invSection)
await sleep(700)
const record = page.getByText('Record Kubernetes events locally').first()
await record.waitFor({ timeout: 10000 })
for (let i = 0; i < 12; i++) {
  const b = await record.boundingBox()
  if (b && b.y < H * 0.45) break
  await r.wheel(140, 8, W * 0.6, H * 0.6)
  await sleep(200)
}
await sleep(400)
await r.mark('history', page.getByText('Stored only on this machine', { exact: false }).first())
await r.hover(record, 800)
await sleep(5200)

// --- CLI -------------------------------------------------------------------------
await page.keyboard.press('Escape')
await sleep(600)
const homeBtn = page.getByRole('button', { name: 'Contexts', exact: true })
await r.mark('home-cli', homeBtn)
await r.click(homeBtn)
await sleep(1500)
const cliBtn = page.locator('.klew-cli-anchor > button').first()
await r.click(cliBtn)
await sleep(1600)
const openTerm = page.getByRole('button', { name: 'Open terminal', exact: true })
await r.mark('cli-popover', page.locator('.klew-cli-code').first())
await r.click(openTerm)
await page.locator('.xterm').first().waitFor({ timeout: 20000 })
await r.waitTerminal((t) => /klew investigate/.test(t), 20000)
await sleep(1500)
const maxBtn = page.locator('button.stream-icon-btn[aria-label="Maximize"]').first()
await maxBtn.waitFor({ state: 'visible', timeout: 10000 })
await r.click(maxBtn, { ms: 600 })
await sleep(800)
// The pre-filled line can draw garbled until zsh redraws it.
await page.locator('.xterm').last().click({ position: { x: 400, y: 300 } }).catch(() => {})
await page.keyboard.press('Control+L')
await sleep(900)
let line = (await r.terminalText()).split('\n').find((l) => l.includes('klew investigate')) || ''
const nsMatch = line.match(/-n\s+(\S+)/)
if (!nsMatch || nsMatch[1] !== NS) {
  await page.keyboard.press('End')
  const drop = line.trimEnd().length - line.trimEnd().lastIndexOf(' ') - 1 + (line.endsWith(' ') ? 1 : 0)
  for (let i = 0; i < drop; i++) {
    await page.keyboard.press('Backspace')
    await sleep(70)
  }
  if (!nsMatch) await r.type(' -n ')
  await r.type(NS, 110)
}
await sleep(600)
await r.mark('cli-run', page.locator('.xterm').last())
await page.keyboard.press('Enter')
await r.waitTerminal((t) => /Some lists are cut short|Next checks/.test(t) && /❯\s*$/.test(t.trimEnd()), 90000)
await sleep(1200)
// Scroll back to the verdict at the top of the report.
const term = await page.locator('.xterm').last().boundingBox()
await r.mark('cli-top')
await r.wheel(-4000, 40, term.x + term.width * 0.5, term.y + term.height * 0.5)
await sleep(5000)
await r.mark('cli-scroll')
await r.wheel(700, 30)
await sleep(3500)
await r.wheel(6000, 20)
await sleep(600)

// --- MCP --------------------------------------------------------------------------
await r.type('clear', 60)
await page.keyboard.press('Enter')
await sleep(700)
// Claude runs in its own empty folder rather than the home directory.
await r.type('cd ~/klew-demo', 60)
await page.keyboard.press('Enter')
await sleep(900)
await r.mark('mcp-add')
await r.type('claude mcp add --scope user klew -- "$(command -v klew)" mcp', 55)
await sleep(500)
await page.keyboard.press('Enter')
await r.waitTerminal((t) => /Added stdio MCP server|already exists/.test(t), 30000)
await sleep(2200)
await r.mark('claude-start')
await r.type('claude', 90)
await page.keyboard.press('Enter')
// First run in this folder asks to trust it ("No, exit" is preselected);
// pick "Yes, I trust this folder" and move on (trimmed in the edit).
const isReady = (t) => !/trust this folder/i.test(t) && /for shortcuts|Try "|Welcome|Claude Code v?\d/i.test(t)
const first = await r.waitTerminal((t) => /trust this folder/i.test(t) || isReady(t), 60000)
if (/trust this folder/i.test(first)) {
  await r.mark('trust')
  await sleep(800)
  await page.keyboard.press('ArrowDown')
  await sleep(400)
  await page.keyboard.press('Enter')
  await r.waitTerminal(isReady, 60000)
}
await sleep(1500)
await r.mark('ask')
await r.type(QUESTION, 55)
await sleep(500)
await page.keyboard.press('Enter')

// Approve klew tool calls as they come; finish once Claude is idle.
let idleSince = 0
let sawWork = false
const deadline = Date.now() + 240000
while (Date.now() < deadline) {
  const t = await r.terminalText()
  if (/Do you want to proceed\?/.test(t)) {
    await r.mark('approve')
    await sleep(900)
    await page.keyboard.press('Enter')
    await sleep(1500)
    idleSince = 0
    continue
  }
  const busy = /esc to interrupt/i.test(t)
  if (busy) {
    sawWork = true
    idleSince = 0
  } else if (sawWork) {
    idleSince ||= Date.now()
    if (Date.now() - idleSince > 3500) break
  }
  await sleep(300)
}
await r.mark('answer')
await sleep(6000)
await r.mark('end')
await r.finish()
