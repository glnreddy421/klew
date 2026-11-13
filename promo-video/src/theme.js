import { loadFont as loadInter } from '@remotion/google-fonts/Inter'
import { loadFont as loadMono } from '@remotion/google-fonts/JetBrainsMono'
import { loadFont as loadDisplay } from '@remotion/google-fonts/SpaceGrotesk'

const inter = loadInter('normal', { weights: ['400', '500', '600', '700'], subsets: ['latin'] })
const mono = loadMono('normal', { weights: ['400', '500'], subsets: ['latin'] })
const display = loadDisplay('normal', { weights: ['400', '500', '600', '700'], subsets: ['latin'] })

// Mirrors the desktop app's dark theme (cmd/klew-desktop/frontend/src/style.css).
export const theme = {
  bg: '#0a0d11',
  surface: '#171b21',
  border: '#2d3540',
  text: '#e8eaed',
  textSecondary: '#9aa3af',
  accent: '#1d8fe1',
  ok: '#2ecc71',
  sans: `${inter.fontFamily}, -apple-system, sans-serif`,
  mono: `${mono.fontFamily}, ui-monospace, Menlo, monospace`,
  // Title and end cards (and the Klew wordmark).
  display: `${display.fontFamily}, ${inter.fontFamily}, sans-serif`,
}

export const FPS = 30
