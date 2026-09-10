import type { CSSProperties } from 'react'
import type { WindowDays } from '../types'
import { haptic } from '../lib/telegram'

const OPTIONS: WindowDays[] = [7, 30, 90]

export function WindowSwitch({
  value,
  onChange,
}: {
  value: WindowDays
  onChange: (d: WindowDays) => void
}) {
  const index = OPTIONS.indexOf(value)
  return (
    <div className="switch stage" style={{ '--stage': 1 } as CSSProperties} role="group" aria-label="Rentang waktu">
      <span
        className="switch-thumb"
        aria-hidden="true"
        style={{ transform: `translateX(${index * 100}%)` }}
      />
      {OPTIONS.map((d) => (
        <button
          key={d}
          aria-pressed={d === value}
          onClick={() => {
            if (d === value) return
            haptic.selection()
            onChange(d)
          }}
        >
          {d} hari
        </button>
      ))}
    </div>
  )
}
