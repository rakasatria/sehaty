import type { Summary, WindowDays } from '../types'
import { emptySummary, mockSummaries } from '../mock'
import { tg } from './telegram'

const params = new URLSearchParams(location.search)
const wantEmpty = params.has('empty')

/**
 * How this request proves who is asking.
 *
 * Inside Telegram it is the launch string, which the client signs with the bot
 * token and the server verifies. In a browser it is the signed token from the
 * one-hour dashboard link, which the page was opened with. Nothing is stored,
 * and neither credential is ever placed in a URL we construct.
 */
function credentials(): Record<string, string> {
  const h: Record<string, string> = { accept: 'application/json' }
  if (tg?.initData) h['x-telegram-init-data'] = tg.initData
  const token = params.get('t') ?? location.pathname.match(/^\/d\/([\w.-]+)/)?.[1]
  if (token) h['x-dashboard-token'] = token
  return h
}

export class NotAuthorised extends Error {}
export class Unreachable extends Error {}

/**
 * Fetch the summary for a window. Every figure in the response is already
 * formatted by the Go server; this function selects nothing and computes
 * nothing.
 *
 * THE FALLBACK IS DEVELOPMENT ONLY, and that matters more than it looks. This
 * used to fall through to src/mock.ts whenever the request failed, which meant a
 * rejected or broken request rendered invented numbers indistinguishable from a
 * real record. For an app whose entire premise is that it never shows a figure
 * nobody measured, silently substituting fiction on error is the worst available
 * failure. In a built bundle a failure now surfaces as a failure.
 */
export async function getSummary(days: WindowDays): Promise<Summary> {
  if (import.meta.env.DEV && wantEmpty) return withLatency(emptySummary(days))

  let res: Response
  try {
    res = await fetch(`/api/summary?days=${days}`, { headers: credentials() })
  } catch (err) {
    if (import.meta.env.DEV) return withLatency(mockSummaries[days])
    throw new Unreachable('the server could not be reached')
  }

  if (res.status === 401 || res.status === 403) {
    throw new NotAuthorised('this record is not open to this session')
  }
  if (!res.ok || !(res.headers.get('content-type') ?? '').includes('json')) {
    if (import.meta.env.DEV) return withLatency(mockSummaries[days])
    throw new Unreachable(`the server answered ${res.status}`)
  }
  return (await res.json()) as Summary
}

/** A short, honest latency in dev so the arrival choreography stays visible. */
async function withLatency<T>(value: T): Promise<T> {
  await new Promise((r) => setTimeout(r, 260))
  return value
}
