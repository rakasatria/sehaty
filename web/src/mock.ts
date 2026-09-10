import type { RecentEntry, SeriesPoint, Summary, WindowDays } from './types'

/**
 * Stand-in for the Go server, used when /api/summary is unreachable
 * (npm run dev without the backend) and for the demo.
 *
 * NOTE: this module is the ONLY place on the client that does arithmetic,
 * because it is playing the server's role — the aggregation, rounding,
 * Indonesian formatting and coordinate math below all mirror what the Go
 * side does before the JSON leaves it. Nothing under src/components/ or
 * src/lib/ computes a number.
 */

const MINUS = '−' // an explicit minus sign, never a hyphen
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const WEEKDAYS = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']

function fmtKg(kg: number): string {
  return kg.toFixed(1).replace('.', ',')
}
function fmtSigned(delta: number): string {
  const v = Math.abs(delta).toFixed(1).replace('.', ',')
  return delta < 0 ? `${MINUS}${v}` : `+${v}`
}
function fmtThousands(n: number): string {
  return Math.round(n)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}
function shortDate(d: Date): string {
  return `${d.getDate()} ${MONTHS[d.getMonth()]}`
}
function longDate(d: Date): string {
  return `${WEEKDAYS[d.getDay()]} ${d.getDate()} ${MONTHS[d.getMonth()]}`
}

// Deterministic noise, so the demo record is the same every run.
function noise(i: number): number {
  return Math.sin(i * 12.9898) * 0.5 + Math.sin(i * 4.1414) * 0.5
}

type Day = {
  date: Date
  ago: number // days before today
  weighKg: number | null
  trained: { name: string; sets: number; minutes: number } | null
  cardioMin: number | null
  foodKcal: number | null
  foodProteinG: number | null
  meals: string[]
}

const TRAINING_NAMES = ['Upper A', 'Lower A', 'Upper B', 'Lower B']
const MEALS = [
  'Nasi ayam bakar + lalapan',
  'Soto ayam, nasi setengah',
  'Nasi padang — rendang, sayur nangka',
  'Gado-gado, tanpa kerupuk',
  'Pecel lele + nasi',
  'Oatmeal, pisang, telur rebus',
  'Bubur ayam tanpa kerupuk',
  'Nasi goreng kampung',
  'Sate ayam 10 tusuk, lontong',
  'Ikan bakar + nasi merah',
]

function buildDays(total: number): Day[] {
  const today = new Date()
  const days: Day[] = []
  let kg = 80.6 // 90 days ago
  for (let ago = total - 1; ago >= 0; ago--) {
    const i = total - ago
    const date = new Date(today)
    date.setDate(today.getDate() - ago)
    kg += -0.036 + noise(i) * 0.22 // slow fat-loss drift with real-life wobble
    const dow = date.getDay()
    const weighs = dow === 1 || dow === 3 || dow === 5 || dow === 6 // most mornings, not all
    const lifts = (dow === 1 || dow === 2 || dow === 4 || dow === 5) && noise(i * 3) > -0.72
    const cardio = (dow === 3 || dow === 6) && noise(i * 7) > -0.5
    const logsFood = noise(i * 11) > -0.85 // misses a day here and there — a fact, not a failure
    days.push({
      date,
      ago,
      weighKg: weighs ? Math.round(kg * 10) / 10 : null,
      trained: lifts
        ? {
            name: TRAINING_NAMES[(i + dow) % TRAINING_NAMES.length],
            sets: 12 + Math.round(Math.abs(noise(i * 5)) * 7),
            minutes: 34 + Math.round(Math.abs(noise(i * 6)) * 18),
          }
        : null,
      cardioMin: cardio ? 20 + Math.round(Math.abs(noise(i * 9)) * 15) : null,
      foodKcal: logsFood ? 1780 + Math.round(noise(i * 13) * 260) : null,
      foodProteinG: logsFood ? 104 + Math.round(noise(i * 17) * 18) : null,
      meals: logsFood ? [MEALS[i % MEALS.length]] : [],
    })
  }
  return days
}

const ALL_DAYS = buildDays(90)

function seriesFor(days: Day[]): SeriesPoint[] {
  const weighed = days.filter((d) => d.weighKg !== null)
  if (weighed.length === 0) return []
  const values = weighed.map((d) => d.weighKg as number)
  const lo = Math.min(...values)
  const hi = Math.max(...values)
  const pad = Math.max((hi - lo) * 0.22, 0.4)
  const top = hi + pad
  const span = hi - lo + pad * 2
  const first = weighed[0].date.getTime()
  const last = weighed[weighed.length - 1].date.getTime()
  const range = Math.max(last - first, 1)
  return weighed.map((d) => ({
    date: shortDate(d.date),
    label: `${fmtKg(d.weighKg as number)} kg`,
    x: weighed.length === 1 ? 0.5 : (d.date.getTime() - first) / range,
    y: (top - (d.weighKg as number)) / span,
  }))
}

function recentFor(days: Day[]): RecentEntry[] {
  const out: RecentEntry[] = []
  for (let i = days.length - 1; i >= 0 && out.length < 14; i--) {
    const d = days[i]
    const when = longDate(d.date)
    if (d.weighKg !== null) {
      out.push({ date: when, what: 'Berat badan', detail: `${fmtKg(d.weighKg)} kg, pagi` })
    }
    if (d.trained) {
      out.push({
        date: when,
        what: 'Latihan beban',
        detail: `${d.trained.name} — ${d.trained.sets} set · ${d.trained.minutes} menit`,
      })
    }
    if (d.cardioMin !== null) {
      out.push({ date: when, what: 'Kardio', detail: `Sepeda statis ${d.cardioMin} menit` })
    }
    for (const meal of d.meals) {
      out.push({ date: when, what: 'Makan', detail: meal })
    }
  }
  return out
}

function summaryFor(windowDays: WindowDays): Summary {
  const days = ALL_DAYS.filter((d) => d.ago < windowDays)
  const weighed = days.filter((d) => d.weighKg !== null)
  const latest = weighed[weighed.length - 1]
  const kcals = days.map((d) => d.foodKcal).filter((v): v is number => v !== null)
  const proteins = days.map((d) => d.foodProteinG).filter((v): v is number => v !== null)
  const trainDays = days.filter((d) => d.trained !== null)
  const cardioTotal = days.reduce((sum, d) => sum + (d.cardioMin ?? 0), 0)
  const hasCardio = days.some((d) => d.cardioMin !== null)
  return {
    name: 'Raka Satria',
    goal: 'menurunkan lemak',
    equipment: ['dumbbell 2×10 kg', 'pull-up bar', 'sepeda statis'],
    window: { days: windowDays, label: `${windowDays} hari terakhir` },
    weight: {
      latest: latest
        ? { value: fmtKg(latest.weighKg as number), unit: 'kg', date: shortDate(latest.date) }
        : null,
      series: seriesFor(days),
      change:
        weighed.length > 1
          ? fmtSigned((weighed[weighed.length - 1].weighKg as number) - (weighed[0].weighKg as number))
          : null,
    },
    training: {
      days: trainDays.length > 0 ? trainDays.length : null,
      sets: trainDays.length > 0 ? trainDays.reduce((s, d) => s + (d.trained?.sets ?? 0), 0) : null,
      cardioMinutes: hasCardio ? cardioTotal : null,
    },
    food: {
      daysLogged: kcals.length > 0 ? kcals.length : null,
      kcal: kcals.length > 0 ? `±${fmtThousands(kcals.reduce((a, b) => a + b, 0) / kcals.length)} kkal` : null,
      protein:
        proteins.length > 0
          ? `${Math.round(proteins.reduce((a, b) => a + b, 0) / proteins.length)} g`
          : null,
    },
    recent: recentFor(days),
    limitations: ['Lutut kanan — hindari gerakan melompat (Feb 2026)', 'Punggung bawah — beban deadlift dibatasi'],
  }
}

export const mockSummaries: Record<WindowDays, Summary> = {
  7: summaryFor(7),
  30: summaryFor(30),
  90: summaryFor(90),
}

/** A record that has only just been created: nothing logged yet. */
export function emptySummary(windowDays: WindowDays): Summary {
  return {
    name: 'Raka Satria',
    goal: '',
    equipment: [],
    window: { days: windowDays, label: `${windowDays} hari terakhir` },
    weight: { latest: null, series: [], change: null },
    training: { days: null, sets: null, cardioMinutes: null },
    food: { daysLogged: null, kcal: null, protein: null },
    recent: [],
    limitations: [],
  }
}
