import { useSyncExternalStore } from 'react'
import { flushSync } from 'react-dom'

/**
 * Motion tokens (mirrored in tokens.css). One vocabulary for the whole app:
 *   - things ARRIVE with EASE_OUT (fast start, long settle — feels answered)
 *   - things LEAVE quickly and quietly
 *   - nothing loops after the page settles
 */
export const EASE_OUT = 'cubic-bezier(0.22, 1, 0.36, 1)'
export const EASE_IN_OUT = 'cubic-bezier(0.65, 0, 0.35, 1)'

const rmQuery = window.matchMedia('(prefers-reduced-motion: reduce)')

export function prefersReducedMotion(): boolean {
  return rmQuery.matches
}

export function useReducedMotion(): boolean {
  return useSyncExternalStore(
    (cb) => {
      rmQuery.addEventListener('change', cb)
      return () => rmQuery.removeEventListener('change', cb)
    },
    () => rmQuery.matches,
  )
}

type DocumentWithVT = Document & {
  startViewTransition?: (cb: () => void) => { finished: Promise<void> }
}

/**
 * Run a state update inside a View Transition when the browser supports it
 * and the person has not asked for reduced motion; otherwise just update.
 * `dir` lets the CSS choreograph forward and backward differently.
 */
export function withViewTransition(dir: 'fwd' | 'back', update: () => void) {
  const doc = document as DocumentWithVT
  if (prefersReducedMotion() || !doc.startViewTransition) {
    update()
    return
  }
  document.documentElement.dataset.vtDir = dir
  const transition = doc.startViewTransition(() => {
    flushSync(update)
  })
  transition.finished.finally(() => {
    delete document.documentElement.dataset.vtDir
  })
}
