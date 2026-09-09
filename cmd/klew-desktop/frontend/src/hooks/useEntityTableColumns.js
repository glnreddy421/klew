import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  allSelectableColumns,
  resolveVisibleColumnIds,
  saveColumnPreferences,
} from '../lib/entityTableColumns.js'

export function useEntityTableColumns({ kind, kindGroup, browseScope }) {
  const resolvedKind = kind || kindGroup?.kind || ''
  const [selectedIds, setSelectedIds] = useState(null)

  useEffect(() => {
    setSelectedIds(null)
  }, [resolvedKind])

  const selectable = useMemo(
    () => allSelectableColumns(resolvedKind, kindGroup, browseScope),
    [resolvedKind, kindGroup, browseScope],
  )

  const visibleIds = useMemo(
    () => resolveVisibleColumnIds({
      kind: resolvedKind,
      kindGroup,
      browseScope,
      selectedIds,
    }),
    [resolvedKind, kindGroup, browseScope, selectedIds],
  )

  const columns = useMemo(
    () => visibleIds
      .map((id) => selectable.find((col) => col.id === id))
      .filter(Boolean),
    [selectable, visibleIds],
  )

  const setVisibleIds = useCallback((ids) => {
    const next = resolveVisibleColumnIds({
      kind: resolvedKind,
      kindGroup,
      browseScope,
      selectedIds: ids,
    })
    setSelectedIds(next)
    if (resolvedKind) saveColumnPreferences(resolvedKind, next)
  }, [resolvedKind, kindGroup, browseScope])

  return {
    columns,
    selectable,
    visibleIds,
    setVisibleIds,
  }
}
