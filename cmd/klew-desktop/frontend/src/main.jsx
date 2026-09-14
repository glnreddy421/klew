import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import { ErrorBoundary } from './components/ErrorBoundary.jsx'
import { initTheme } from './lib/themes'
import { initUiFont } from './lib/fonts'
import { loadPreferences } from './lib/preferences'
import { applyTopbarScale } from './lib/topbarScale'
import { initSettingsCache } from './lib/settingsCache'
import './style.css'

async function boot() {
  initTheme()
  await initSettingsCache()
  await initUiFont()
  applyTopbarScale(loadPreferences().topbarScale)
  ReactDOM.createRoot(document.getElementById('root')).render(
    <React.StrictMode>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </React.StrictMode>,
  )
}

boot()
