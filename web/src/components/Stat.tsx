import { Swap } from './Swap'

/**
 * One figure on its baseline. `value` is displayed exactly as it arrived;
 * null is rendered as an em-dash with "belum dicatat" — absence as a fact,
 * never a zero.
 */
export function Stat({
  label,
  value,
  unit,
  dir,
  compact = false,
  delayMs = 0,
}: {
  label: string
  value: string | number | null
  unit?: string
  dir: 'up' | 'down' | 'none'
  compact?: boolean
  delayMs?: number
}) {
  const shown = value === null ? null : String(value)
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <span className={`stat-value num${compact ? ' compact' : ''}`}>
        <Swap id={shown ?? '—'} dir={dir} delayMs={delayMs}>
          {shown === null ? (
            <>
              <span className="absent-mark">—</span>
              <span className="absent-note">belum dicatat</span>
            </>
          ) : (
            <>
              {shown}
              {unit && <span className="stat-unit">{unit}</span>}
            </>
          )}
        </Swap>
      </span>
    </div>
  )
}
