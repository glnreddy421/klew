import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ListLogStreams,
  LogStreamLines,
  PauseLogStream,
  ResumeLogStream,
  StartLogStream,
  StopLogStream,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { appendLines } from '../lib/logStream'

const IDLE = { id: '', active: false, paused: false, spec: { targets: [] }, sources: [], dropped: 0, notices: [] }
const EMPTY_LINES = []

/**
 * Manages multiple concurrent container log streams, keyed by stream ID.
 * Lines arrive in batches on "logs:lines" (with Batch.id) and status updates
 * on "logs:status" (with Status.id).
 * maxLogRequests (Settings → Max concurrent log follows) applies per tab.
 */
export function useLogStreams({ maxLogRequests = 0, onRestored } = {}) {
  const [streams, setStreams] = useState({})
  const [lastError, setLastError] = useState('')
  /** Per-stream sequence watermarks for lines cleared by the user. */
  const clearedSeqRef = useRef(new Map())
  const maxRef = useRef(maxLogRequests)
  maxRef.current = maxLogRequests
  const onRestoredRef = useRef(onRestored)
  onRestoredRef.current = onRestored

  useEffect(() => {
    let cancelled = false
    Promise.resolve(ListLogStreams?.())
      .then(async (list) => {
        if (cancelled || !Array.isArray(list) || !list.length) return
        const entries = await Promise.all(
          list.map(async (st) => {
            const id = st?.id || st?.streamId || ''
            if (!id) return null
            let buffered = []
            try {
              buffered = (await LogStreamLines(id)) || []
            } catch {
              buffered = []
            }
            return [id, {
              id,
              status: { ...IDLE, ...st, id },
              lines: appendLines([], buffered),
              busy: false,
              error: '',
            }]
          }),
        )
        if (cancelled) return
        const valid = entries.filter(Boolean)
        if (!valid.length) return
        setStreams((prev) => {
          const next = { ...prev }
          for (const [id, entry] of valid) {
            next[id] = next[id]
              ? { ...entry, lines: appendLines(entry.lines, next[id].lines) }
              : entry
          }
          return next
        })
        onRestoredRef.current?.(valid.map(([, entry]) => entry.status))
      })
      .catch(() => {})

    const offLines = EventsOn('logs:lines', (batch) => {
      const id = batch?.id || batch?.streamId || ''
      if (!id) return
      const clearedAt = clearedSeqRef.current.get(id) || 0
      const fresh = (batch?.lines || []).filter((l) => l.seq > clearedAt)
      if (!fresh.length && batch?.dropped == null) return
      setStreams((prev) => {
        const cur = prev[id] || { id, status: { ...IDLE, id }, lines: [], busy: false, error: '' }
        const nextLines = fresh.length ? appendLines(cur.lines, fresh) : cur.lines
        const nextDropped = batch?.dropped != null ? batch.dropped : cur.status.dropped
        if (nextLines === cur.lines && nextDropped === cur.status.dropped) return prev
        return {
          ...prev,
          [id]: {
            ...cur,
            lines: nextLines,
            status: nextDropped === cur.status.dropped
              ? cur.status
              : { ...cur.status, dropped: nextDropped },
          },
        }
      })
    })

    const offStatus = EventsOn('logs:status', (st) => {
      const id = st?.id || st?.streamId || ''
      if (!id) return
      setStreams((prev) => {
        const cur = prev[id]
        if (!cur && !st?.active) return prev
        const base = cur || { id, status: { ...IDLE, id }, lines: [], busy: false, error: '' }
        return {
          ...prev,
          [id]: {
            ...base,
            status: { ...IDLE, ...st, id },
          },
        }
      })
    })

    return () => {
      cancelled = true
      offLines?.()
      offStatus?.()
    }
  }, [])

  const setStreamPatch = useCallback((id, patch) => {
    if (!id) return
    setStreams((prev) => {
      const cur = prev[id]
      if (!cur) return prev
      return { ...prev, [id]: { ...cur, ...(typeof patch === 'function' ? patch(cur) : patch) } }
    })
  }, [])

  const start = useCallback(async (opts = {}) => {
    setLastError('')
    const mergeId = opts.mergeIntoActive && opts.streamId ? String(opts.streamId) : ''
    if (mergeId) setStreamPatch(mergeId, { busy: true, error: '' })
    try {
      const st = await StartLogStream({
        maxLogRequests: maxRef.current || 0,
        ...opts,
      })
      const id = st?.id || st?.streamId || mergeId
      if (!id) return st
      if (!mergeId) {
        clearedSeqRef.current.set(id, 0)
      }
      let buffered = []
      try {
        buffered = (await LogStreamLines(id)) || []
      } catch {
        buffered = []
      }
      const clearedAt = clearedSeqRef.current.get(id) || 0
      const cleanBuffered = buffered.filter((l) => l.seq > clearedAt)
      setStreams((prev) => {
        const existing = prev[id]
        const baseLines = mergeId && existing ? existing.lines : []
        const mergedLines = appendLines(baseLines, cleanBuffered)
        const finalLines = !mergeId && existing?.lines?.length
          ? appendLines(mergedLines, existing.lines)
          : mergedLines
        return {
          ...prev,
          [id]: {
            id,
            status: { ...IDLE, ...st, id },
            lines: finalLines,
            busy: false,
            error: '',
          },
        }
      })
      return st
    } catch (err) {
      const msg = String(err?.message || err)
      setLastError(msg)
      if (mergeId) setStreamPatch(mergeId, { busy: false, error: msg })
      return null
    }
  }, [setStreamPatch])

  const stop = useCallback(async (id, { keepEntry = false } = {}) => {
    if (!id) return
    setStreamPatch(id, { busy: true, error: '' })
    try {
      await StopLogStream(id)
    } catch (err) {
      setStreamPatch(id, { busy: false, error: String(err?.message || err) })
      return
    }
    clearedSeqRef.current.delete(id)
    setStreams((prev) => {
      if (!prev[id]) return prev
      if (keepEntry) {
        const prevSpec = prev[id]?.status?.spec || { targets: [] }
        return {
          ...prev,
          [id]: {
            id,
            status: { ...IDLE, id, spec: prevSpec },
            lines: [],
            busy: false,
            error: '',
          },
        }
      }
      const next = { ...prev }
      delete next[id]
      return next
    })
  }, [setStreamPatch])

  const stopAll = useCallback(async () => {
    try {
      await StopLogStream('')
    } catch {
      /* ignore */
    }
    clearedSeqRef.current.clear()
    setStreams({})
  }, [])

  const pause = useCallback(async (id) => {
    if (!id) return null
    setStreamPatch(id, { busy: true, error: '' })
    try {
      await PauseLogStream(id)
      setStreamPatch(id, { busy: false })
      return true
    } catch (err) {
      setStreamPatch(id, { busy: false, error: String(err?.message || err) })
      return null
    }
  }, [setStreamPatch])

  const resume = useCallback(async (id) => {
    if (!id) return null
    setStreamPatch(id, { busy: true, error: '' })
    try {
      await ResumeLogStream(id)
      setStreamPatch(id, { busy: false })
      return true
    } catch (err) {
      setStreamPatch(id, { busy: false, error: String(err?.message || err) })
      return null
    }
  }, [setStreamPatch])

  /** Clears the tab's lines only; the stream keeps running. */
  const clear = useCallback((id) => {
    if (!id) return
    setStreams((prev) => {
      const cur = prev[id]
      if (!cur) return prev
      if (cur.lines.length) {
        clearedSeqRef.current.set(id, cur.lines[cur.lines.length - 1].seq)
      }
      return { ...prev, [id]: { ...cur, lines: [] } }
    })
  }, [])

  const getStream = useCallback((id) => {
    const entry = id ? streams[id] : null
    return {
      id: id || '',
      status: entry?.status || { ...IDLE, id: id || '' },
      lines: entry?.lines || EMPTY_LINES,
      busy: Boolean(entry?.busy),
      error: entry?.error || '',
      start: (opts = {}) => start({ ...opts, streamId: id }),
      stop: () => stop(id, { keepEntry: true }),
      close: () => stop(id, { keepEntry: false }),
      pause: () => pause(id),
      resume: () => resume(id),
      clear: () => clear(id),
    }
  }, [streams, start, stop, pause, resume, clear])

  const hasActiveStream = Object.values(streams).some((s) => Boolean(s?.status?.active))

  return {
    streams,
    hasActiveStream,
    lastError,
    clearError: useCallback(() => setLastError(''), []),
    start,
    stop,
    stopAll,
    pause,
    resume,
    clear,
    getStream,
  }
}

export const useLogStream = useLogStreams
