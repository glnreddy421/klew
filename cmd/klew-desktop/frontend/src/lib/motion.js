/**
 * Shared motion: one calm timing for every transition in the app.
 * CSS uses the same values through --motion-* and --ease-* (style.css).
 */

export const MOTION_MS = 180
export const EASE_OUT = 'cubic-bezier(0.2, 0, 0, 1)'

export function prefersReducedMotion() {
  try {
    return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  } catch {
    return false
  }
}

/**
 * Fades an element in with a small rise, without remounting it, so state and
 * scroll position survive. Cancels a running enter on the same element.
 */
export function animateIn(el, { y = 4, duration = MOTION_MS } = {}) {
  if (!el?.animate || prefersReducedMotion()) return null
  el.getAnimations?.().forEach((a) => {
    if (a.id === 'klew-enter') a.cancel()
  })
  const anim = el.animate(
    [
      { opacity: 0, transform: `translateY(${y}px)` },
      { opacity: 1, transform: 'none' },
    ],
    { duration, easing: EASE_OUT },
  )
  anim.id = 'klew-enter'
  return anim
}
