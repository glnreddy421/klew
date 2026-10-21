import { createContext, useContext } from 'react'

const EventHistoryContext = createContext(null)

export function EventHistoryProvider({ value, children }) {
  return (
    <EventHistoryContext.Provider value={value}>
      {children}
    </EventHistoryContext.Provider>
  )
}

/** Recording state from useEventHistory, or null outside the provider. */
export function useEventHistoryContext() {
  return useContext(EventHistoryContext)
}
