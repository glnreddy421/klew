/** Control chars, zero-width, and other non-printing noise. */
const INVISIBLE = /[\u0000-\u001F\u007F-\u009F\u200B-\u200D\uFEFF]/g

/** Keep chars useful for kind/name and free-text workload search. */
const UNUSUAL = /[^\w./\-:@]+/g

/**
 * Normalize the investigation search string: trim, drop invisible chars,
 * and strip punctuation/whitespace that cannot belong in a workload query.
 * Returns '' when nothing meaningful remains (browse whole namespace).
 */
export function normalizeInvestigationQuery(raw) {
  return String(raw ?? '')
    .normalize('NFKC')
    .replace(INVISIBLE, '')
    .replace(UNUSUAL, '')
    .trim()
}

export function isBlankInvestigationQuery(raw) {
  return normalizeInvestigationQuery(raw) === ''
}
