import {
  useId,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import type { SeriesPoint } from '../types'
import { haptic } from '../lib/telegram'

/**
 * The weight trace. Coordinates arrive pre-computed from the server
 * (x, y normalised 0..1); this component only maps them onto the box —
 * geometry, not measurement. On arrival the line draws itself once
 * (stroke-dashoffset, compositor-friendly) and the latest point settles.
 *
 * Scrubbing snaps to MEASURED points — the crosshair never rests between
 * two weigh-ins, because between them there is no number. Each snap ticks
 * the haptic engine once, like a detent on an instrument dial.
 */
export function WeightChart({ series, delayMs }: { series: SeriesPoint[]; delayMs: number }) {
  const gradId = useId()
  const boxRef = useRef<HTMLDivElement>(null)
  const rectRef = useRef<DOMRect | null>(null)
  const engagedRef = useRef(false)
  const releaseTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [active, setActive] = useState<number | null>(null)
  const [engaged, setEngaged] = useState(false)

  const n = series.length
  const latest = n > 0 ? series[n - 1] : null

  const linePath =
    n > 1
      ? series.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x * 100} ${p.y * 100}`).join(' ')
      : null
  const areaPath = linePath
    ? `${linePath} L${series[n - 1].x * 100} 100 L${series[0].x * 100} 100 Z`
    : null

  function nearestIndex(fraction: number): number {
    let best = 0
    let bestDist = Infinity
    for (let i = 0; i < n; i++) {
      const d = Math.abs(series[i].x - fraction)
      if (d < bestDist) {
        bestDist = d
        best = i
      }
    }
    return best
  }

  function moveTo(index: number) {
    clearTimeout(releaseTimer.current)
    setEngaged(true)
    setActive((prev) => {
      if (prev !== index) haptic.selection()
      return index
    })
  }

  function scrub(clientX: number) {
    const rect = rectRef.current
    if (!rect || rect.width === 0) return
    moveTo(nearestIndex((clientX - rect.left) / rect.width))
  }

  function engage(e: ReactPointerEvent<HTMLDivElement>) {
    if (n === 0) return
    rectRef.current = boxRef.current?.getBoundingClientRect() ?? null
    engagedRef.current = true
    e.currentTarget.setPointerCapture(e.pointerId)
    scrub(e.clientX)
  }

  function release() {
    engagedRef.current = false
    // Let the crosshair layer settle out (160ms fade) before unmounting it.
    setEngaged(false)
    clearTimeout(releaseTimer.current)
    releaseTimer.current = setTimeout(() => setActive(null), 220)
  }

  function onKeyDown(e: ReactKeyboardEvent) {
    if (n === 0) return
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    rectRef.current = boxRef.current?.getBoundingClientRect() ?? null
    const step = e.key === 'ArrowLeft' ? -1 : 1
    const next = Math.max(0, Math.min(n - 1, (active ?? n - 1) + step))
    moveTo(next)
  }

  const rect = rectRef.current
  const point = active !== null ? series[active] : null
  const px = point && rect ? point.x * rect.width : 0
  const py = point && rect ? point.y * rect.height : 0
  const edge = point ? (point.x < 0.14 ? 'edge-l' : point.x > 0.86 ? 'edge-r' : '') : ''

  return (
    <div className="chart-wrap">
      <div
        ref={boxRef}
        className="chart"
        style={{ '--chart-delay': `${delayMs}ms` } as CSSProperties}
        role="img"
        aria-label={
          latest
            ? `Grafik berat badan. Terakhir ${latest.label}, ${latest.date}. Geser untuk membaca titik.`
            : 'Grafik berat badan. Belum ada data.'
        }
        tabIndex={n > 0 ? 0 : -1}
        onPointerDown={engage}
        onPointerMove={(e) => engagedRef.current && scrub(e.clientX)}
        onPointerUp={release}
        onPointerCancel={release}
        onKeyDown={onKeyDown}
        onBlur={release}
      >
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          <defs>
            <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="currentColor" stopOpacity="0.16" />
              <stop offset="1" stopColor="currentColor" stopOpacity="0" />
            </linearGradient>
          </defs>
          {areaPath && (
            <path
              className="chart-area"
              d={areaPath}
              fill={`url(#${gradId})`}
              style={{ color: 'var(--accent)' }}
            />
          )}
          {linePath && (
            <path className="chart-line" d={linePath} pathLength={1} vectorEffect="non-scaling-stroke" />
          )}
        </svg>

        <div className="chart-baseline" aria-hidden="true" />

        {latest && (
          <span
            className="chart-dot latest"
            aria-hidden="true"
            style={{ left: `${latest.x * 100}%`, top: `${latest.y * 100}%` }}
          />
        )}

        {n === 0 && (
          <div className="chart-empty">
            <p>Belum ada data berat badan.</p>
            <p className="sub">Timbangan pertama yang kamu catat akan tergambar di sini.</p>
          </div>
        )}

        <div className={`scrub-layer${point && engaged ? ' on' : ''}`} aria-hidden="true">
          {point && rect && (
            <>
              <div className="chart-cross" style={{ transform: `translate3d(${px}px,0,0)` }} />
              <span
                className="chart-dot scrub"
                style={{ transform: `translate3d(${px}px,${py}px,0)` }}
              />
              <div className="chart-tipline" style={{ transform: `translate3d(${px}px,0,0)` }}>
                <span className={`chart-tip num ${edge}`}>
                  {point.label}
                  <span className="tip-date">{point.date}</span>
                </span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
