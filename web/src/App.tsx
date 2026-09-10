import { useCallback, useEffect, useRef, useState } from 'react'
import type { Summary, WindowDays } from './types'
import { getSummary, NotAuthorised } from './lib/api'
import { withViewTransition } from './lib/motion'
import { haptic, inTelegram, tg } from './lib/telegram'
import { SummaryView } from './components/Summary'
import { DayView } from './components/DayView'
import { LogView } from './components/LogView'
import { Locked } from './components/Locked'

type Route = { v: 'summary' } | { v: 'log' } | { v: 'day'; date: string }

const initialRoute: Route =
  new URLSearchParams(location.search).get('view') === 'log' ? { v: 'log' } : { v: 'summary' }

export default function App() {
  const [route, setRoute] = useState<Route>(initialRoute)
  const [days, setDays] = useState<WindowDays>(30)
  const [summary, setSummary] = useState<Summary | null>(null)
  const [dir, setDir] = useState<'up' | 'down'>('up')
  const [printing, setPrinting] = useState(true)
  const [failed, setFailed] = useState<'auth' | 'unreachable' | null>(null)
  const cache = useRef(new Map<WindowDays, Summary>())

  // ---- data ----
  useEffect(() => {
    let live = true
    const cached = cache.current.get(days)
    if (cached) {
      setSummary(cached)
      return
    }
    getSummary(days)
      .then((s) => {
        if (!live) return
        cache.current.set(days, s)
        setFailed(null)
        setSummary(s)
      })
      .catch((err) => {
        // A record that cannot be loaded says so. It must never fall back to
        // anything that looks like data, because a plausible wrong number is
        // the one failure this app exists to prevent.
        if (!live) return
        setFailed(err instanceof NotAuthorised ? 'auth' : 'unreachable')
      })
    return () => {
      live = false
    }
  }, [days])

  // The record prints itself exactly once, on first arrival.
  const hasData = summary !== null
  useEffect(() => {
    if (!hasData) return
    const t = setTimeout(() => setPrinting(false), 1800)
    return () => clearTimeout(t)
  }, [hasData])

  // ---- navigation: history-backed, view-transitioned ----
  const navigate = useCallback((next: Route, vtDir: 'fwd' | 'back', push: boolean) => {
    withViewTransition(vtDir, () => {
      setRoute(next)
      if (push) history.pushState({ route: next }, '')
      if (vtDir === 'fwd') window.scrollTo(0, 0)
    })
  }, [])

  useEffect(() => {
    history.replaceState({ route: initialRoute }, '')
    const onPop = (e: PopStateEvent) => {
      const next: Route = (e.state?.route as Route) ?? { v: 'summary' }
      withViewTransition('back', () => setRoute(next))
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  // Telegram's native back button on subviews.
  useEffect(() => {
    const bb = tg?.BackButton
    if (!bb || route.v === 'summary') {
      bb?.hide()
      return
    }
    const back = () => history.back()
    bb.onClick(back)
    bb.show()
    return () => {
      bb.offClick(back)
      bb.hide()
    }
  }, [route])

  const goBack = useCallback(() => history.back(), [])

  const changeDays = useCallback(
    (next: WindowDays) => {
      setDir(next > days ? 'up' : 'down')
      setDays(next)
    },
    [days],
  )

  if (failed !== null) {
    return (
      <div className="app state-fail" role="alert">
        <p className="fail-title">
          {failed === 'auth' ? 'Catatan ini tidak terbuka di sesi ini.' : 'Catatanmu tidak bisa dimuat.'}
        </p>
        <p className="fail-body">
          {failed === 'auth'
            ? 'Buka lagi lewat Sehaty di Telegram, atau minta tautan baru — tautan dasbor berlaku satu jam.'
            : 'Server tidak menjawab. Tidak ada angka yang ditampilkan di sini kecuali datang dari catatanmu sendiri.'}
        </p>
      </div>
    )
  }

  if (summary === null) {
    // The ground is already painted; the record arrives as one piece.
    return <div className="app" aria-busy="true" />
  }

  return (
    <div className="app" {...(printing ? { 'data-enter': '' } : {})}>
      {route.v === 'summary' && (
        <SummaryView
          data={summary}
          days={days}
          dir={dir}
          chartDelayMs={printing ? 560 : 60}
          onDays={changeDays}
          onOpenDay={(date) => {
            navigate({ v: 'day', date }, 'fwd', true)
          }}
          onOpenLog={() => {
            haptic.impact('light')
            navigate({ v: 'log' }, 'fwd', true)
          }}
        />
      )}
      {route.v === 'summary' && <Locked items={summary.locked} />}
      {route.v === 'day' && (
        <DayView date={route.date} entries={summary.recent} onBack={goBack} showBackLink={!inTelegram} />
      )}
      {route.v === 'log' && (
        <LogView entries={summary.recent} onBack={goBack} showBackLink={!inTelegram} />
      )}
    </div>
  )
}
