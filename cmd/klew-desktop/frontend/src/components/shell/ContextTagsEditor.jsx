import { useEffect, useRef, useState } from 'react'
import { MAX_CONTEXT_TAGS, normalizeTag } from '../../lib/clusterContexts.js'

/**
 * Inline tags for one kube context — chips plus an add field.
 */
export function ContextTagsEditor({
  tags = [],
  disabled = false,
  onAdd,
  onRemove,
}) {
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState('')
  const inputRef = useRef(null)
  const canAdd = !disabled && tags.length < MAX_CONTEXT_TAGS && Boolean(onAdd)

  useEffect(() => {
    if (adding) inputRef.current?.focus()
  }, [adding])

  function commit() {
    const tag = normalizeTag(draft)
    if (!tag) {
      setDraft('')
      setAdding(false)
      return
    }
    onAdd?.(tag)
    setDraft('')
    setAdding(false)
  }

  return (
    <div className="context-tags" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
      {tags.map((tag) => (
        <span key={tag} className="context-tag">
          <span className="context-tag-label">{tag}</span>
          {onRemove && (
            <button
              type="button"
              className="context-tag-remove"
              title={`Remove tag ${tag}`}
              aria-label={`Remove tag ${tag}`}
              disabled={disabled}
              onClick={() => onRemove(tag)}
            >
              ×
            </button>
          )}
        </span>
      ))}
      {canAdd && !adding && (
        <button
          type="button"
          className="context-tag-add"
          onClick={() => setAdding(true)}
          disabled={disabled}
        >
          + Tag
        </button>
      )}
      {canAdd && adding && (
        <input
          ref={inputRef}
          type="text"
          className="context-tag-input"
          value={draft}
          maxLength={32}
          placeholder="tag"
          aria-label="Add tag"
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              commit()
            } else if (e.key === 'Escape') {
              e.preventDefault()
              setDraft('')
              setAdding(false)
            }
          }}
        />
      )}
    </div>
  )
}
