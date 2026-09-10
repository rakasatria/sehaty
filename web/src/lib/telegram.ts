/**
 * A thin, typed wrapper over window.Telegram.WebApp.
 *
 * We use the raw injected object rather than @telegram-apps/sdk: the four
 * things Sehaty needs (theme, viewport, haptics, back button) are a few
 * dozen lines, and the bundle stays honest.
 */

type HapticStyle = 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'

interface TelegramWebApp {
  initData: string
  platform: string
  colorScheme: 'light' | 'dark'
  themeParams: Record<string, string>
  ready(): void
  expand(): void
  disableVerticalSwipes?: () => void
  setHeaderColor?: (color: string) => void
  onEvent(event: string, cb: () => void): void
  offEvent(event: string, cb: () => void): void
  HapticFeedback?: {
    impactOccurred(style: HapticStyle): void
    selectionChanged(): void
    notificationOccurred(type: 'error' | 'success' | 'warning'): void
  }
  BackButton?: {
    isVisible: boolean
    show(): void
    hide(): void
    onClick(cb: () => void): void
    offClick(cb: () => void): void
  }
}

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp }
  }
}

export const tg: TelegramWebApp | undefined = window.Telegram?.WebApp

/** True only when actually running inside a Telegram client. */
export const inTelegram = !!tg && tg.platform !== 'unknown' && tg.initData !== ''

function applyTheme() {
  if (!tg) return
  const root = document.documentElement
  // Older clients don't inject the CSS variables themselves; copy them in.
  for (const [key, value] of Object.entries(tg.themeParams)) {
    root.style.setProperty(`--tg-theme-${key.replace(/_/g, '-')}`, value)
  }
  root.style.colorScheme = tg.colorScheme
  root.dataset.scheme = tg.colorScheme
}

export function initTelegram() {
  if (!tg) return
  applyTheme()
  tg.onEvent('themeChanged', applyTheme)
  tg.ready()
  tg.expand()
  // The weight chart is scrubbed horizontally; keep a stray vertical wobble
  // from collapsing the Mini App sheet mid-gesture.
  tg.disableVerticalSwipes?.()
  tg.setHeaderColor?.('bg_color')
}

/**
 * Haptics: only on real interactions — window change, scrub tick,
 * opening a record. Never decorative.
 */
export const haptic = {
  selection() {
    tg?.HapticFeedback?.selectionChanged()
  },
  impact(style: HapticStyle = 'light') {
    tg?.HapticFeedback?.impactOccurred(style)
  },
}
