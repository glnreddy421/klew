import { createContext, useContext } from 'react'

const ShellInspectorContext = createContext(null)

export function ShellInspectorProvider({ value, children }) {
  return (
    <ShellInspectorContext.Provider value={value}>
      {children}
    </ShellInspectorContext.Provider>
  )
}

export function useShellInspector() {
  return useContext(ShellInspectorContext)
}
