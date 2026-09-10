import type { CSSProperties } from 'react'
import type { Summary, WindowDays } from '../types'
import { Section } from './Section'
import { Stat } from './Stat'
import { Swap } from './Swap'
import { WeightChart } from './WeightChart'
import { WindowSwitch } from './WindowSwitch'
import { RecentList } from './RecentList'

/**
 * The record itself. Section order is the order a physician would read:
 * identity, the primary trace (weight), work done, intake, the raw log,
 * standing limitations.
 */
export function SummaryView({
  data,
  days,
  dir,
  chartDelayMs,
  onDays,
  onOpenDay,
  onOpenLog,
}: {
  data: Summary
  days: WindowDays
  dir: 'up' | 'down'
  chartDelayMs: number
  onDays: (d: WindowDays) => void
  onOpenDay: (date: string) => void
  onOpenLog: () => void
}) {
  const { weight, training, food } = data

  return (
    <main>
      <header className="masthead" style={{ '--stage': 0 } as CSSProperties}>
        <div className="stage">
          <span className="eyebrow">Sehaty — rekam kesehatan pribadi</span>
          <h1 className="name serif">{data.name}</h1>
          {data.goal !== '' && <p className="meta">Tujuan: {data.goal}</p>}
        </div>
        <WindowSwitch value={days} onChange={onDays} />
      </header>

      <Section
        label="Berat badan"
        stage={2}
        aside={
          weight.latest && (
            <Swap id={weight.latest.date} dir={dir} className="meta">
              <span className="meta num">per {weight.latest.date}</span>
            </Swap>
          )
        }
      >
        <div className="weight-now">
          <span className="weight-kg serif num">
            <Swap id={weight.latest?.value ?? '—'} dir={dir}>
              {weight.latest ? (
                <>
                  {weight.latest.value}
                  <span className="stat-unit">{weight.latest.unit}</span>
                </>
              ) : (
                <span className="absent-mark">—</span>
              )}
            </Swap>
          </span>
          <span className="weight-change num">
            <Swap id={`${weight.change ?? '—'}|${data.window.label}`} dir={dir} delayMs={40}>
              {weight.change !== null ? (
                <>
                  <strong>{weight.change} kg</strong> · {data.window.label}
                </>
              ) : weight.latest === null ? (
                <>belum dicatat</>
              ) : (
                <>satu titik data · {data.window.label}</>
              )}
            </Swap>
          </span>
        </div>
        <Swap id={String(days)} dir="none" block>
          <WeightChart series={weight.series} delayMs={chartDelayMs} />
        </Swap>
      </Section>

      <Section label="Latihan" stage={3}>
        <div className="stats">
          <Stat label="Hari latihan" value={training.days} dir={dir} delayMs={0} />
          <Stat label="Total set" value={training.sets} dir={dir} delayMs={40} />
          <Stat label="Kardio" value={training.cardioMinutes} unit="menit" dir={dir} delayMs={80} />
        </div>
      </Section>

      <Section label="Makan" stage={4}>
        <div className="stats">
          <Stat label="Hari tercatat" value={food.daysLogged} dir={dir} delayMs={0} />
          <Stat label="Energi rata-rata" value={food.kcal} compact dir={dir} delayMs={40} />
          <Stat label="Protein rata-rata" value={food.protein} compact dir={dir} delayMs={80} />
        </div>
        <p className="footnote">Sehaty tidak menghitung target kalori — itu ranah ahli gizimu.</p>
      </Section>

      <Section label="Catatan terakhir" stage={5}>
        <RecentList entries={data.recent} onOpenDay={onOpenDay} onOpenLog={onOpenLog} />
      </Section>

      <Section label="Batasan tercatat" stage={6}>
        {data.limitations.length === 0 ? (
          <p className="footnote" style={{ margin: 0 }}>
            Tidak ada batasan tercatat.
          </p>
        ) : (
          data.limitations.map((item, i) => (
            <div key={i} className="limit-item">
              {item}
            </div>
          ))
        )}
      </Section>

      <footer className="foot stage" style={{ '--stage': 7 } as CSSProperties}>
        {data.equipment.length > 0 && <p>Peralatan: {data.equipment.join(' · ')}</p>}
        <p>Semua angka dihitung dan dibulatkan di server — tidak ada yang dihitung di halaman ini.</p>
        <p>Sehaty · rekam pribadi di server sendiri.</p>
      </footer>
    </main>
  )
}
