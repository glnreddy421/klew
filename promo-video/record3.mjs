// Video 2, CLI segment only: Home → CLI → Open terminal → klew investigate on
// holdout, then scroll back to the verdict at the top of the report.
//   node record3.mjs && node assemble.mjs out/raw3 public/take3.mp4 src/markers3.json
import { createRecorder, sleep, W, H } from './lib/recorder.mjs'

const NS = 'holdout'
const r = await createRecorder({ out: 'out/raw3' })
const { page } = r

await r.open()
const cliBtn = page.locator('.klew-cli-anchor > button').first()
await cliBtn.waitFor({ timeout: 60000 })
await sleep(2500)
await r.start()

await r.mark('home-cli', cliBtn)
await sleep(800)
await r.click(cliBtn, { ms: 900 })
await sleep(1600)
await r.mark('cli-popover', page.locator('.klew-cli-code').first())
await r.click(page.getByRole('button', { name: 'Open terminal', exact: true }))
await r.waitTerminal((t) => /klew investigate/.test(t), 20000)
await sleep(1200)
const maxBtn = page.locator('button.stream-icon-btn[aria-label="Maximize"]').first()
for (let i = 0; i < 3 && !(await page.locator('.app-root.stream-maximized').count()); i++) {
  await r.click(maxBtn, { ms: 600 })
  await sleep(900)
}
// Focus the shell again; the click left focus on the button.
const xt = await page.locator('.xterm').last().boundingBox()
await r.click(page.locator('.xterm').last(), { ms: 500 })
await r.glide(xt.x + xt.width * 0.6, xt.y + xt.height * 0.6, 400)
await sleep(300)

const line = (await r.terminalText()).split('\n').find((l) => l.includes('klew investigate')) || ''
const m = line.match(/-n\s+(\S+)/)
if (!m || m[1] !== NS) {
  await page.keyboard.press('End')
  if (m) {
    const drop = m[1].length + (line.trimEnd().endsWith(m[1]) && line.endsWith(' ') ? 1 : 0)
    for (let i = 0; i < drop; i++) {
      await page.keyboard.press('Backspace')
      await sleep(80)
    }
  } else {
    await r.type(' -n ')
  }
  await r.type(NS, 110)
}
await sleep(600)
await r.mark('cli-run', page.locator('.xterm').last())
await page.keyboard.press('Enter')
await r.waitTerminal((t) => /Some lists are cut short|Next checks/.test(t) && /❯\s*$/.test(t.trimEnd()), 90000)
await sleep(1500)

// Scroll back until the viewport is at the very top of the scrollback.
const term = await page.locator('.xterm').last().boundingBox()
await r.glide(term.x + term.width * 0.5, term.y + term.height * 0.5, 400)
await r.mark('cli-top')
// Stop once the command line itself (the top of the report) is in view.
for (let i = 0; i < 600; i++) {
  if (/❯ klew investigate/.test(await r.terminalText())) break
  await page.mouse.wheel(0, -240)
  await sleep(16)
}
await r.mark('cli-verdict')
await sleep(6000)
await r.mark('cli-scroll')
for (let i = 0; i < 30; i++) {
  await page.mouse.wheel(0, 40)
  await sleep(60)
}
await sleep(3500)
await r.mark('end')
await r.finish()
