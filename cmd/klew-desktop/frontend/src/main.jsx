import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import { ErrorBoundary } from './components/ErrorBoundary.jsx'
import { initTheme } from './lib/themes'
import { initUiFont } from './lib/fonts'
import { initSettingsCache } from './lib/settingsCache'
import './style.css'

async function boot() {
  initTheme()
  await initSettingsCache()
  await initUiFont()
  ReactDOM.createRoot(document.getElementById('root')).render(
    <React.StrictMode>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </React.StrictMode>,
  )
}

boot()
