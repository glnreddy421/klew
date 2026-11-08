import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Fixed port: if Vite falls back to another port, its HMR client still
  // targets the configured one and the Wails window never live-reloads.
  server: {
    port: 5190,
    strictPort: true,
    hmr: { protocol: 'ws', host: 'localhost', port: 5190 },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
