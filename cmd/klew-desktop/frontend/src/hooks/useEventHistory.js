import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ClearEventHistory,
  GetEventHistorySettings,
  GetEventHistoryStatus,
  SetEventHistorySettings,
  SetEventRecordingPaused,
  SetRecordedContexts,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { normalizeHistorySettings, statusByContext } from '../lib/eventHistory'

const REFRESH_MS = 60_000

function errorText(err, fallback) {
  const msg = String(err?.message || err || '').trim()
  return msg ? `${fallback}: ${msg}` : fallback
}

/**
 * Local event recording state for this window. `recordedContexts` is the
 * per-cluster opt-in list; monitoring pause pauses every recorder; Settings
 * has the global master switch.
 */
export function useEventHistory({ recordedContexts = [], monitoringPaused = false, contexts = [] } = {}) {
  const [statuses, setStatuses] = useState([])
  const [settings, setSettings] = useState(() => normalizeHistorySettings())
  const [busy, setBusy] = useState(false)
  const contextsRef = useRef(contexts)
  contextsRef.current = contexts

  const recordedKey = (recordedContexts || []).join('\u0000')
  const ctxKey = (contexts || []).join('\u0000')

  const refresh = useCallback(() => {
    GetEventHistoryStatus(contextsRef.current || [])
      .then((list) => setStatuses(Array.isArray(list) ? list : []))
      .catch(() => {})
  }, [])

  const loadSettings = useCallback(() => {
    GetEventHistorySettings()
      .then((s) => setSettings(normalizeHistorySettings(s)))
      .catch(() => {})
  }, [])

  useEffect(() => {
    SetRecordedContexts(recordedContexts || [])
      .then(() => refresh())
      .catch(() => {})
    // recordedKey captures recordedContexts content.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [recordedKey, refresh])

  useEffect(() => {
    SetEventRecordingPaused(Boolean(monitoringPaused)).catch(() => {})
  }, [monitoringPaused])

  useEffect(() => {
    refresh()
  }, [ctxKey, refresh])

  useEffect(() => {
    const off = EventsOn('eventHistory', () => refresh())
    const timer = setInterval(refresh, REFRESH_MS)
    loadSettings()
    return () => {
      if (typeof off === 'function') off()
      clearInterval(timer)
    }
  }, [refresh, loadSettings])

  const saveSettings = useCallback((patch) => {
    const next = normalizeHistorySettings({ ...settings, ...patch })
    setSettings((prev) => ({ ...prev, ...next }))
    setBusy(true)
    return SetEventHistorySettings(next.enabled, next.retentionDays, next.maxMB, next.eventTTLMinutes)
      .then((s) => setSettings(normalizeHistorySettings(s)))
      .catch((err) => setSettings((prev) => ({ ...prev, error: errorText(err, 'Could not save event history settings') })))
      .finally(() => {
        setBusy(false)
        refresh()
      })
  }, [settings, refresh])

  const clearHistory = useCallback(() => {
    setBusy(true)
    return ClearEventHistory()
      .then((s) => setSettings(normalizeHistorySettings(s)))
      .catch((err) => setSettings((prev) => ({ ...prev, error: errorText(err, 'Could not clear event history') })))
      .finally(() => {
        setBusy(false)
        refresh()
      })
  }, [refresh])

  const byContext = useMemo(() => statusByContext(statuses), [statuses])

  return {
    statuses,
    byContext,
    settings,
    busy,
    refresh,
    loadSettings,
    saveSettings,
    clearHistory,
  }
}
