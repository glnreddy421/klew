import { createContext, useContext } from 'react'

const TerminalLaunchContext = createContext(null)

export function TerminalLaunchProvider({ launch, children }) {
  return (
    <TerminalLaunchContext.Provider value={launch}>
      {children}
    </TerminalLaunchContext.Provider>
  )
}

export function useTerminalLaunch() {
  return useContext(TerminalLaunchContext)
}
