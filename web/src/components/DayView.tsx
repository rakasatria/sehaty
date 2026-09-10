import type { CSSProperties } from 'react'
import type { RecentEntry } from '../types'

/**
 * One day of the record. Entries are selected by date-string match —
 * selection, never computation. The title carries the shared-element
 * name so the tapped date morphs into it.
 */
export function DayView({
  date,
  entries,
  onBack,
  showBackLink,
}: {
  date: string
  entries: RecentEntry[]
  onBack: () => void
  showBackLink: boolean
}) {
  const own = entries.filter((e) => e.date === date)
  return (
    <main>
      <div className="subview-top">
        {showBackLink && (
          <button className="back-link" onClick={onBack}>
            ‹ Ringkasan
          </button>
        )}
      </div>
      <h1 className="subview-title serif" style={{ viewTransitionName: 'day-date' } as CSSProperties}>
        {date}
      </h1>
      <p className="subview-sub">Yang tercatat hari itu.</p>
      <div className="section-rule" />
      <div className="rows">
        {own.length === 0 ? (
          <div className="rows-empty">
            <p>Tidak ada catatan pada hari ini.</p>
            <p className="sub">Hari kosong dibiarkan kosong — itu juga bagian dari rekam.</p>
          </div>
        ) : (
          own.map((entry, i) => (
            <div key={i} className="entry rise" style={{ '--i': i } as CSSProperties}>
              <span className="entry-what">{entry.what}</span>
              <span className="entry-detail">{entry.detail}</span>
            </div>
          ))
        )}
      </div>
    </main>
  )
}
