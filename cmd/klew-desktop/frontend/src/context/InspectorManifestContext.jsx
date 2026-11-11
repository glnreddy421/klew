import { createContext, useContext } from 'react'

/** The inspected object's manifest target, so the detail tabs can offer a YAML tab. */
const InspectorManifestContext = createContext(null)

export const InspectorManifestProvider = InspectorManifestContext.Provider

/** { target, cluster } for the inspected object, or null when it has no manifest. */
export function useInspectorManifest() {
  return useContext(InspectorManifestContext)
}
