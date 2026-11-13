// MCP segment only, recorded at a smaller viewport so the terminal text comes
// out larger once scaled to 1080p: add klew to Claude, ask why holdout fails,
// then ask for the ledger-db Secret to show values are never returned.
//   node record4.mjs && node assemble.mjs out/raw4 public/take4.mp4 src/markers4.json
import { createRecorder, sleep } from './lib/recorder.mjs'

const QUESTIONS = [
  ['general', 'Why is the holdout namespace on docker-desktop failing? Use klew and show me stats.'],
  ['billing', 'Why is the billing workload in the holdout namespace on docker-desktop failing? Use klew and show me stats.'],
  ['secrets', 'Can you extract the secrets from this workload or namespace? Use klew MCP only.'],
]

const r = await createRecorder({ out: 'out/raw4', width: 1440, height: 810 })
const { page } = r

await r.open()
const cliBtn = page.locator('.klew-cli-anchor > button').first()
await cliBtn.waitFor({ timeout: 60000 })
await sleep(2000)
await r.start()

// Setup (cut in the edit): open a terminal, maximize, neutral prompt.
// The CLI button opens a terminal with a command typed in; it is cleared below.
await r.click(cliBtn, { ms: 500 })
await sleep(1000)
await r.click(page.getByRole('button', { name: 'Open terminal', exact: true }), { ms: 500 })
await page.locator('.xterm').last().waitFor({ timeout: 20000 })
await r.waitTerminal((t) => /\S/.test(t), 20000)
await sleep(1500)
const maxBtn = page.locator('button.stream-icon-btn[aria-label="Maximize"]').first()
for (let i = 0; i < 3 && !(await page.locator('.app-root.stream-maximized').count()); i++) {
  await r.click(maxBtn, { ms: 400 })
  await sleep(800)
}
await r.click(page.locator('.xterm').last(), { ms: 400 })
const xt = await page.locator('.xterm').last().boundingBox()
await r.glide(xt.x + xt.width * 0.7, xt.y + xt.height * 0.75, 300)
await page.keyboard.press('Control+U')
await sleep(600)
// Neutral prompt (ASCII only) so the machine name stays off screen; retry until it shows.
for (let i = 0; i < 4; i++) {
  // Claude gets only the klew MCP server here: none of the user's other servers or plugins.
  await page.keyboard.type("PS1='%F{39}%1~%f > '; RPROMPT=''; claude() { if [ $# -eq 0 ]; then command claude --strict-mcp-config --mcp-config ~/klew-demo/.mcp-klew.json; else command claude \"$@\"; fi; }; cd ~/klew-demo; clear", { delay: 15 })
  await page.keyboard.press('Enter')
  await sleep(1500)
  const t = await r.terminalText()
  if (/klew-demo >/.test(t) && !/@/.test(t)) break
  await page.keyboard.press('Control+U')
  await sleep(500)
}
// --- On camera ---------------------------------------------------------------
await r.mark('mcp-add')
await r.type('claude mcp add --scope user klew -- "$(command -v klew)" mcp', 50)
await sleep(400)
await page.keyboard.press('Enter')
await r.waitTerminal((t) => /Added stdio MCP server|already exists/.test(t), 30000)
await sleep(2000)
await r.mark('claude-start')
await r.type('claude', 90)
await page.keyboard.press('Enter')
const isReady = (t) => !/trust this folder/i.test(t) && /for shortcuts|Try "|Welcome|Claude Code v?\d/i.test(t)
const first = await r.waitTerminal((t) => /trust this folder/i.test(t) || isReady(t), 60000)
if (/trust this folder/i.test(first)) {
  await r.mark('trust')
  await sleep(600)
  await page.keyboard.press('ArrowDown')
  await sleep(300)
  await page.keyboard.press('Enter')
  await r.waitTerminal(isReady, 60000)
}
await sleep(1500)

async function ask(label, q) {
  await r.mark(`ask-${label}`)
  await r.type(q, 50)
  await sleep(500)
  await page.keyboard.press('Enter')
  let idleSince = 0
  let sawWork = false
  const deadline = Date.now() + 240000
  while (Date.now() < deadline) {
    const t = await r.terminalText()
    if (/Do you want to proceed\?/.test(t)) {
      await r.mark(`approve-${label}`)
      await sleep(900)
      await page.keyboard.press('Enter')
      await sleep(1500)
      idleSince = 0
      continue
    }
    if (/esc to interrupt/i.test(t)) {
      sawWork = true
      idleSince = 0
    } else if (sawWork) {
      idleSince ||= Date.now()
      if (Date.now() - idleSince > 3000) break
    }
    await sleep(300)
  }
  await r.mark(`answer-${label}`)
}

for (const [label, q] of QUESTIONS) {
  await ask(label, q)
  await sleep(7000)
}
await r.mark('end')
await r.finish()
