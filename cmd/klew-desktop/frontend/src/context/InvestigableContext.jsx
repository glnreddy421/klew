import { createContext, useContext } from 'react'

const InvestigableContext = createContext(null)

/**
 * Shares the engine's investigable types and the action that investigates
 * one object, so every Resources surface marks the same types.
 * value: { index, loaded, investigate(row) }
 */
export function InvestigableProvider({ value, children }) {
  return <InvestigableContext.Provider value={value}>{children}</InvestigableContext.Provider>
}

export function useInvestigable() {
  return useContext(InvestigableContext)
}
