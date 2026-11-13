// Still screenshots of the running app (wails dev) for klew-ui, in dark and light.
//   node shots.mjs                 → out/shots/<view>-<theme>.png for both themes
//   node shots.mjs dark logs       → one theme, some views
// Views: browse, compare, logs, overview, failures, evidence, patterns, graph.
import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'

const APP = 'http://localhost:34115'
const OUT = path.resolve('out/shots')
const W = 1600
const H = 900
const CHROME = path.join(
  os.homedir(),
  'Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
)
fs.mkdirSync(OUT, { recursive: true })
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const args = process.argv.slice(2)
const themes = args.filter((a) => a === 'dark' || a === 'light')
const only = args.filter((a) => a !== 'dark' && a !== 'light')
const want = (...names) => !only.length || names.some((n) => only.includes(n))

const browser = await chromium.launch({ executablePath: fs.existsSync(CHROME) ? CHROME : undefined })

for (const theme of themes.length ? themes : ['dark', 'light']) {
  const context = await browser.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: 2, colorScheme: theme })
  await context.addInitScript((t) => {
    try { localStorage.setItem('klew-desktop-theme', t) } catch {}
  }, theme)
  const page = await context.newPage()
  const shot = async (name) => {
    await page.mouse.move(W - 4, H - 4)
    await sleep(500)
    await page.screenshot({ path: path.join(OUT, `${name}-${theme}.png`) })
    console.log('shot', name, theme)
  }
  const debug = async (e) => {
    await page.screenshot({ path: path.join(OUT, `debug-${theme}.png`) })
    throw e
  }

  await page.goto(APP, { waitUntil: 'domcontentloaded' })
  const tile = page.getByTitle('Open docker-desktop')
  await tile.waitFor({ timeout: 60000 })
  await sleep(2500)
  await tile.click()
  await page.getByRole('heading', { name: 'Overview' }).first().waitFor({ timeout: 30000 }).catch(() => {})
  await sleep(2000)

  if (want('browse', 'compare', 'logs')) {
    await page.getByRole('button', { name: /^Deployments(\s+\d+)?$/ }).click()
    await sleep(1500)
    // The app remembers the last scope (holdout after an investigation); list klew-lab too.
    // The app keeps the last investigation's namespace as the browse scope;
    // set the scope to klew-lab in the investigate bar, then go back to Watch.
    const setScope = async (ns) => {
      await page.getByTitle('Investigate one namespace or object (⌘K)').click()
      await sleep(800)
      const q = page.getByRole('group', { name: 'What to investigate' })
      await q.getByRole('button', { name: /^Scope:/ }).click()
      await page.getByPlaceholder('Search or type namespace…').fill(ns)
      await sleep(600)
      await page.getByRole('option', { name: new RegExp(ns) }).first().click()
      await sleep(600)
      await page.locator('.investigate-bar-run').click()
      await page.getByRole('navigation', { name: 'Investigation results' }).waitFor({ timeout: 30000 })
      await sleep(3000)
      await page.getByRole('button', { name: 'Stop', exact: true }).click().catch(() => {})
      await sleep(800)
      await page.getByRole('button', { name: 'Watch', exact: true }).first().click()
      await sleep(1500)
      await page.getByRole('button', { name: /^Deployments(\s+\d+)?$/ }).click()
      await sleep(1500)
    }
    await setScope('klew-lab')
    if (process.env.NSDEBUG) await debug(new Error('ns debug'))
    const search = page.getByPlaceholder('Search deployments…')

    if (want('browse')) {
      await page.locator('.resource-nav-kind-row', { hasText: /^Pods/ }).first().click()
      await sleep(1800)
      const podRow = page.getByRole('row').filter({ hasText: /gateway-/ }).first()
      await podRow.getByText(/gateway-/).first().click({ timeout: 10000 }).catch(debug)
      await sleep(2500)
      await shot('browse')
      await page.getByRole('button', { name: /^Deployments(\s+\d+)?$/ }).click()
      await sleep(1500)
    }

    if (want('compare')) {
      await search.fill(process.env.CMP || 'payment-api')
      await sleep(1200)
      const row = page.getByRole('row').filter({ hasText: process.env.CMP || 'payment-api' }).first()
      await row.getByText(process.env.CMP || 'payment-api', { exact: true }).first().click({ timeout: 10000 }).catch(debug)
      await sleep(2000)
      await page.getByRole('tab', { name: /^Compare/ }).click()
      await page.locator('.compare-diff').first().waitFor({ timeout: 20000 }).catch(() => {})
      await sleep(1500)
      await shot('compare')
      await search.fill('')
      await sleep(1000)
      await page.keyboard.press('Escape')
    }

    if (want('logs')) {
      for (const name of ['audit-log', 'gateway', 'notifier']) {
        const cb = page.getByRole('checkbox', { name: `Include Deployment/${name} in live log stream` })
        await cb.scrollIntoViewIfNeeded()
        await cb.click()
        await sleep(300)
      }
      await page.locator('.entity-table-scroll').first().evaluate((el) => { el.scrollTop = 0 })
      await sleep(400)
      await page.locator('.entity-table-stream-logs-btn').first().click()
      await sleep(2500)
      await page.locator('button.stream-icon-btn[aria-label="Maximize"]').first().click({ timeout: 10000 }).catch(debug)
      await sleep(8000)
      await shot('logs')
      await page.locator('button.stream-icon-btn[aria-label^="Close"]').first().click().catch(() => {})
      await sleep(800)
    }
  }

  if (want('overview', 'failures', 'evidence', 'patterns', 'graph')) {
    await page.getByTitle('Investigate one namespace or object (⌘K)').click()
    await sleep(900)
    const query = page.getByRole('group', { name: 'What to investigate' })
    await query.getByRole('button', { name: /^Scope:/ }).click()
    await page.getByPlaceholder('Search or type namespace…').fill('holdout')
    await sleep(600)
    await page.getByRole('option', { name: /holdout/ }).first().click()
    await sleep(500)
    await page.locator('.investigate-bar-run').click()
    const tabs = page.getByRole('navigation', { name: 'Investigation results' })
    await tabs.waitFor({ timeout: 30000 })
    await sleep(9000)
    if (want('overview')) await shot('overview')
    for (const [name, re] of [['failures', /^Failures/], ['evidence', /^Evidence/], ['patterns', /^Patterns/], ['graph', /^Graph/]]) {
      if (!want(name)) continue
      await tabs.getByRole('button', { name: re }).click()
      await sleep(name === 'graph' ? 5000 : 3500)
      // Correlated signals need an event and a log burst in the same minutes; wait for a link.
      for (let i = 0; name === 'evidence' && i < 18; i++) {
        if (!(await page.getByText('No linked patterns yet').count())) break
        await sleep(10000)
        await tabs.getByRole('button', { name: /^Failures/ }).click()
        await sleep(800)
        await tabs.getByRole('button', { name: re }).click()
        await sleep(2500)
      }
      await shot(name)
    }
  }
  await context.close()
}

await browser.close()
