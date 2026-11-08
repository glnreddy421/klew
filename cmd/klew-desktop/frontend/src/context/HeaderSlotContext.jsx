import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'

/**
 * Slots in the Watch / Investigate bar that views fill with their own header
 * content (a breadcrumb, a toolbar), so the bar, the page header and the
 * resource toolbar share one row instead of stacking three.
 *
 * The bar registers its slot elements; HeaderSlot renders children into one
 * through a portal, keeping them in their own component's React tree, or in
 * place when the bar is not shown.
 */
const HeaderSlotContext = createContext(null)

export function HeaderSlotProvider({ children }) {
  const [slots, setSlots] = useState({})
  // Stable, so slot ref callbacks do not change and re-register on every render.
  const register = useCallback((name) => (node) => {
    setSlots((prev) => (prev[name] === node ? prev : { ...prev, [name]: node }))
  }, [])
  const value = useMemo(() => ({ slots, register }), [slots, register])
  return <HeaderSlotContext.Provider value={value}>{children}</HeaderSlotContext.Provider>
}

/** Ref callback for the bar's slot element called name. */
export function useHeaderSlotRef(name) {
  const register = useContext(HeaderSlotContext)?.register
  return useMemo(() => register?.(name), [register, name])
}

/** Renders children into the bar's slot called name, or in place without one. */
export function HeaderSlot({ name, children }) {
  const node = useContext(HeaderSlotContext)?.slots[name]
  return node ? createPortal(children, node) : children
}
