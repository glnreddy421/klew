import { createContext, useContext } from 'react'

const LogStreamContext = createContext(null)

/**
 * Lets Resources surfaces start a log stream and report what is selected,
 * so ⌘⇧L can stream the current pod selection.
 * value: { streamResources(resources), streamPodLogs({ kind, name, namespace, container }), setSelection(source, resources) }
 */
export function LogStreamProvider({ value, children }) {
  return <LogStreamContext.Provider value={value}>{children}</LogStreamContext.Provider>
}

export function useLogStreamActions() {
  return useContext(LogStreamContext)
}
