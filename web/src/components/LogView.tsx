import { Fragment, type CSSProperties } from 'react'
import type { RecentEntry } from '../types'

/**
 * The full log the server sent, grouped under date headings. Grouping is
 * string equality on pre-humanised dates — order and text both come from
 * the server; nothing is re-sorted or re-derived here.
 */
export function LogView({
  entries,
  onBack,
  showBackLink,
}: {
  entries: RecentEntry[]
  onBack: () => void
  showBackLink: boolean
}) {
  let lastDate: string | null = null
  let block = -1

  return (
    <main>
      <div className="subview-top">
        {showBackLink && (
          <button className="back-link" onClick={onBack}>
            ‹ Ringkasan
          </button>
        )}
      </div>
      <h1 className="subview-title serif">Catatan</h1>
      <p className="subview-sub">Semua yang terkirim dari server, terbaru dulu.</p>
      <div className="section-rule" />
      {entries.length === 0 ? (
        <div className="rows-empty">
          <p>Belum ada yang tercatat.</p>
          <p className="sub">Catatan pertama dari chat akan muncul di sini.</p>
        </div>
      ) : (
        entries.map((entry, i) => {
          const isNewDate = entry.date !== lastDate
          lastDate = entry.date
          if (isNewDate) block++
          return (
            <Fragment key={i}>
              {isNewDate && (
                <h2 className="log-date rise" style={{ '--i': block } as CSSProperties}>
                  {entry.date}
                </h2>
              )}
              <div className="entry rise" style={{ '--i': block } as CSSProperties}>
                <span className="entry-what">{entry.what}</span>
                <span className="entry-detail">{entry.detail}</span>
              </div>
            </Fragment>
          )
        })
      )}
    </main>
  )
}
