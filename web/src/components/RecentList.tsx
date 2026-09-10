import type { RecentEntry } from '../types'
import { haptic } from '../lib/telegram'

/**
 * The last handful of entries. Tapping a row promotes its date into the
 * day view's title via a shared-element view transition: the name
 * `day-date` is stamped onto the tapped date only, at tap time, so the
 * browser morphs exactly that piece of text.
 */
export function RecentList({
  entries,
  onOpenDay,
  onOpenLog,
}: {
  entries: RecentEntry[]
  onOpenDay: (date: string) => void
  onOpenLog: () => void
}) {
  if (entries.length === 0) {
    return (
      <div className="rows-empty">
        <p>Belum ada yang tercatat.</p>
        <p className="sub">Catatan pertama dari chat akan muncul di sini — latihan, makan, atau berat badan.</p>
      </div>
    )
  }

  const shown = entries.slice(0, 5)

  return (
    <>
      <div className="rows">
        {shown.map((entry, i) => (
          <button
            key={`${entry.date}-${entry.what}-${i}`}
            className="row"
            onClick={(e) => {
              const dateEl = e.currentTarget.querySelector<HTMLElement>('.row-date')
              if (dateEl) dateEl.style.viewTransitionName = 'day-date'
              haptic.impact('light')
              onOpenDay(entry.date)
              // If the transition is skipped (unsupported / reduced motion),
              // the name must not linger on a reused row.
              setTimeout(() => {
                if (dateEl) dateEl.style.viewTransitionName = ''
              }, 400)
            }}
          >
            <span className="row-date">{entry.date}</span>
            <span className="row-main">
              <span className="row-what">{entry.what}</span>
              <span className="row-detail">{entry.detail}</span>
            </span>
            <span className="row-chev" aria-hidden="true">
              ›
            </span>
          </button>
        ))}
      </div>
      <button className="rows-more" onClick={onOpenLog}>
        Lihat semua catatan ›
      </button>
    </>
  )
}
