import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { useReducedMotion } from '../lib/motion'

type Held = { id: string; node: ReactNode }

/**
 * Animates the ARRIVAL of new content, never its value: when `id` changes,
 * the old node slips out (150ms) while the new one settles in (240ms,
 * slightly delayed so the two never fight for attention). Nothing is
 * interpolated — at no instant does the screen show a value that was not
 * in the data.
 *
 * `dir` gives the swap a direction: widening the window reads upward,
 * narrowing it reads downward. `delayMs` staggers a row of figures.
 */
export function Swap({
  id,
  dir = 'up',
  block = false,
  delayMs = 0,
  className,
  children,
}: {
  id: string
  dir?: 'up' | 'down' | 'none'
  block?: boolean
  delayMs?: number
  className?: string
  children: ReactNode
}) {
  const reduced = useReducedMotion()
  const heldRef = useRef<Held>({ id, node: children })
  const [prev, setPrev] = useState<Held | null>(null)

  if (heldRef.current.id !== id) {
    const old = heldRef.current
    heldRef.current = { id, node: children }
    if (!reduced) setPrev(old) // render-phase update: old and new paint together
  } else {
    heldRef.current = { id, node: children }
  }

  useEffect(() => {
    if (prev === null) return
    const t = setTimeout(() => setPrev(null), 340 + delayMs)
    return () => clearTimeout(t)
  }, [prev, delayMs])

  return (
    <span
      className={`swap${block ? ' block' : ''}${className ? ` ${className}` : ''}`}
      data-dir={dir}
      style={delayMs ? ({ '--swap-delay': `${delayMs}ms` } as CSSProperties) : undefined}
    >
      {prev !== null && (
        <span key={`old-${prev.id}`} className="swap-old" aria-hidden="true">
          {prev.node}
        </span>
      )}
      <span key={`cur-${id}`} className={prev !== null ? 'swap-new' : undefined}>
        {children}
      </span>
    </span>
  )
}
