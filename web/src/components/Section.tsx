import type { CSSProperties, ReactNode } from 'react'

/**
 * One ruled section of the record: a labelled hairline, then content.
 * `stage` is its position in the entrance choreography.
 */
export function Section({
  label,
  aside,
  stage,
  children,
}: {
  label: string
  aside?: ReactNode
  stage: number
  children: ReactNode
}) {
  return (
    <section className="section" style={{ '--stage': stage } as CSSProperties}>
      <div className="section-head stage">
        <span className="eyebrow">{label}</span>
        {aside}
      </div>
      <div className="section-rule" />
      <div className="section-body stage">{children}</div>
    </section>
  )
}
