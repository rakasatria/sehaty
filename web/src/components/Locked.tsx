import type { LockedCapability } from '../types'

/**
 * What this record cannot do yet, and why.
 *
 * Renders nothing at all when nothing is locked — an empty "everything is unlocked"
 * box is a box that exists only to be dismissed. Every string here comes from the Go
 * capability registry; this component chooses no wording and hides no need.
 */
export function Locked({ items }: { items: LockedCapability[] }) {
  if (items.length === 0) return null

  return (
    <section className="locked" aria-labelledby="locked-heading">
      <h2 id="locked-heading">Belum bisa</h2>
      {items.map((c) => (
        <article key={c.unlocks} className="locked-item">
          <h3>{c.unlocks}</h3>
          <ul>
            {c.needs.map((n) => (
              <li key={n.field}>
                <b>{n.field}</b>
                <span>{n.because}</span>
              </li>
            ))}
          </ul>
        </article>
      ))}
    </section>
  )
}
