import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  entitiesForKind,
  filterEntitiesBySearch,
  pickDefaultKindSelection,
  resolveKindSelection,
  visibleCategories,
} from '../lib/resourceNavigation.js'

const DEFAULT_EXPANDED = new Set(['workloads'])

/**
 * Resource navigation state — accordion-style groups, kind selection.
 */
export function useResourceNavigation(tree, { enabled = true } = {}) {
  const [expandedGroups, setExpandedGroups] = useState(() => new Set(DEFAULT_EXPANDED))
  const [selectedGroupId, setSelectedGroupId] = useState(null)
  const [selectedKind, setSelectedKind] = useState(null)
  const [selectedResourceId, setSelectedResourceId] = useState(null)
  const [showEmptyKinds, setShowEmptyKinds] = useState(true)
  const [entitySearchQuery, setEntitySearchQuery] = useState('')

  const categories = useMemo(
    () => (enabled ? visibleCategories(tree, showEmptyKinds) : []),
    [tree, showEmptyKinds, enabled],
  )

  const selectionRef = useRef({ selectedGroupId, selectedKind, selectedResourceId })
  selectionRef.current = { selectedGroupId, selectedKind, selectedResourceId }

  // Reconcile selection when the tree changes — not on every user click.
  useEffect(() => {
    if (!enabled) return
    const prev = selectionRef.current
    const next = resolveKindSelection(
      tree,
      { groupId: prev.selectedGroupId, kind: prev.selectedKind, resourceId: prev.selectedResourceId },
      showEmptyKinds,
    )
    if (
      next.groupId !== prev.selectedGroupId
      || next.kind !== prev.selectedKind
      || next.resourceId !== prev.selectedResourceId
    ) {
      setSelectedGroupId(next.groupId)
      setSelectedKind(next.kind)
      setSelectedResourceId(next.resourceId)
    }
  }, [tree, showEmptyKinds, enabled])

  // Keep the selected category expanded — do not auto-expand everything.
  useEffect(() => {
    if (!enabled || !selectedGroupId) return
    setExpandedGroups((prev) => {
      if (prev.has(selectedGroupId)) return prev
      const next = new Set(prev)
      next.add(selectedGroupId)
      return next
    })
  }, [selectedGroupId, enabled])

  const entities = useMemo(() => {
    if (!enabled) return []
    return entitiesForKind(tree, selectedGroupId, selectedKind, selectedResourceId)
  }, [tree, selectedGroupId, selectedKind, selectedResourceId, enabled])

  const filteredEntities = useMemo(
    () => filterEntitiesBySearch(entities, entitySearchQuery),
    [entities, entitySearchQuery],
  )

  const selectedKindGroup = useMemo(() => {
    const cat = tree?.categories?.find((c) => c.id === selectedGroupId)
    return cat?.kinds?.find((k) =>
      selectedResourceId ? k.resourceId === selectedResourceId : k.kind === selectedKind,
    ) || null
  }, [tree, selectedGroupId, selectedKind, selectedResourceId])

  const toggleGroup = useCallback((groupId) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(groupId)) next.delete(groupId)
      else next.add(groupId)
      return next
    })
  }, [])

  const selectKind = useCallback((groupId, kind, resourceId) => {
    setSelectedGroupId(groupId)
    setSelectedKind(kind)
    setSelectedResourceId(resourceId || null)
    setEntitySearchQuery('')
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      next.add(groupId)
      return next
    })
  }, [])

  const toggleShowEmpty = useCallback(() => {
    setShowEmptyKinds((v) => !v)
  }, [])

  return {
    categories,
    expandedGroups,
    selectedGroupId,
    selectedKind,
    selectedResourceId,
    selectedKindGroup,
    showEmptyKinds,
    entitySearchQuery,
    entities,
    filteredEntities,
    toggleGroup,
    selectKind,
    toggleShowEmpty,
    setEntitySearchQuery,
    pickDefault: () => pickDefaultKindSelection(tree, showEmptyKinds),
  }
}
