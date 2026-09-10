/**
 * The shape of /api/summary, produced by the Go server.
 *
 * Everything here arrives DISPLAY-READY. Values are pre-formatted strings
 * (Indonesian decimal comma, explicit minus sign), dates are pre-humanised
 * ("Kam 10 Sep"), and chart coordinates are pre-computed, normalised to
 * 0..1 with y measured from the top. The client selects and places; it
 * never adds, averages, rounds or interpolates. `null` means "not
 * recorded" — a fact, never a zero.
 */

export type WindowDays = 7 | 30 | 90

export type Summary = {
  name: string
  goal: string // already humanised, e.g. "menurunkan lemak"
  equipment: string[]
  window: { days: number; label: string } // e.g. "30 hari terakhir"
  weight: {
    latest: { value: string; unit: 'kg'; date: string } | null // value PRE-FORMATTED, e.g. "77,4"
    series: SeriesPoint[]
    change: string | null // "−1,8" with an explicit sign, pre-formatted
  }
  training: {
    days: number | null
    sets: number | null
    cardioMinutes: number | null
  }
  food: {
    daysLogged: number | null
    kcal: string | null // e.g. "±1.930 kkal"
    protein: string | null // e.g. "112 g"
  }
  recent: RecentEntry[]
  limitations: string[] // injuries on record, pre-humanised
  locked: LockedCapability[] // empty when nothing is gated; never null
}

export type SeriesPoint = {
  date: string // pre-humanised, e.g. "12 Agu"
  label: string // pre-formatted display value, e.g. "78,2 kg"
  x: number // 0..1, left → right
  y: number // 0..1, top → bottom (screen coordinates)
}

export type RecentEntry = {
  date: string // pre-humanised, e.g. "Kam 10 Sep"
  what: string // e.g. "Latihan beban"
  detail: string // e.g. "Upper A — 16 set · 42 menit"
}

/**
 * Something the record cannot do yet. Both strings arrive ready to render — the
 * client picks neither the wording nor which needs to show.
 */
export type LockedCapability = {
  unlocks: string
  needs: LockedNeed[]
}

export type LockedNeed = {
  field: string
  because: string
}
