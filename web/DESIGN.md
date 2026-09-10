# Sehaty — design notes

## Visual direction

A **ruled record, not an app**. No card stacks, no chips, no gradient hero: the page
is a single ledger column where every section is a labelled hairline and every value
sits on a baseline, the way figures sit on a printed lab form. Identity moments
(the owner's name, the latest weight, a day's title) are set in the system serif
stack (New York / Georgia / Noto Serif); every figure is tabular-numeral sans.
No webfonts are shipped — the register comes from treatment, not from faces, and
the bundle stays at ~76 kB gzip total with zero animation libraries.

Inside Telegram, every color resolves from the injected `--tg-theme-*` variables,
so the record always harmonises with the person's own theme — including the accent,
which borrows `--tg-theme-link-color` so the weight trace never fights the client.
In a plain browser the fallback palette is Sehaty's own: cool paper `#F4F6F3`,
ink `#1C201D`, a deep leaf-green `#2B6E52` (light) / `#79BD9A` (dark). One accent,
used in exactly three places: the trace, the active links, focus rings.

Absence is drawn as absence: an em-dash on the baseline with *"belum dicatat"* in
small italics. Never a zero, never gray-tinted guilt. The empty chart states a fact
and what will happen: *"Timbangan pertama yang kamu catat akan tergambar di sini."*
Copy is the Jakarta mix the owner actually reads — Bahasa Indonesia structure,
English data terms (`set`, `kkal`) where those are the household words.

## Motion choreography

One vocabulary everywhere: **things arrive with `cubic-bezier(0.22, 1, 0.36, 1)`**
(fast start, long settle — reads as "answered"), things leave in ≤170 ms with plain
`ease-in`, and **nothing loops after the page settles**. Every animated property is
compositor-friendly (`transform`, `opacity`, `stroke-dashoffset`); the only layout
reads happen once per gesture (`getBoundingClientRect` on pointer-down), never per
frame.

### 1. The record prints itself (first arrival only)

Eight stages, 70 ms apart: masthead → window switch → weight → training → food →
log → limitations → footer. Each stage is a 560 ms rise (10 px translate + fade,
ease-out-quint) while its section hairline draws left-to-right (`scaleX 0→1`,
620 ms). The weight trace draws last (`stroke-dashoffset` over 800 ms via
`pathLength=1`, starting at +560 ms), its area wash fades in under the finished
line, and the latest point *settles* (scale 0.3→1, 320 ms) just before the line
completes — the instrument finishing its trace. Total: ~1.4 s, and it runs exactly
once (`data-enter` is removed after 1.8 s so returning from a subview never
replays it). The stagger is the signature; everything else stays quiet.

### 2. Scrubbing the trace

The crosshair **snaps between measured points** (90 ms ease-out on `transform`)
instead of gliding continuously — between two weigh-ins there is no number, so the
cursor refuses to rest there. Each snap fires `HapticFeedback.selectionChanged()`:
a detent on a dial. The tooltip shows the point's pre-formatted `label` and `date`
verbatim. On release the layer settles out over 160 ms rather than popping.
Keyboard: ←/→ walk the points. `touch-action: none` plus Telegram's
`disableVerticalSwipes()` keep the gesture from collapsing the sheet.

### 3. Changing the window (7 / 30 / 90 hari)

The switch thumb slides on `transform` (260 ms). Every figure swaps by **arrival,
never by value**: the old string slips out (150 ms, 8 px) as the new one settles in
(240 ms, delayed 60 ms so they never fight), staggered 40 ms across a row of
figures. The swap is **directional** — widening the window reads upward, narrowing
reads downward — so the gesture and the motion agree. A count-up tween was
rejected outright: interpolated frames would display weights nobody measured.
The chart crossfades out (150 ms) while the new trace draws (800 ms); values
that didn't change don't move at all, which is itself information.

### 4. Between views (summary ⇄ day ⇄ log)

View Transitions API, wrapped in `flushSync`, with graceful fallback (unsupported
browsers and `prefers-reduced-motion` get an instant cut — the record is complete
without motion). Forward: old view exits up 8 px/170 ms, new enters from +14 px/
300 ms; back reverses the vertical grammar so space stays consistent. The tapped
row's **date is stamped `view-transition-name: day-date` at tap time** and morphs
into the day view's serif title — the one shared-element moment, reserved for the
one navigation that is semantically "zooming into a day". Telegram's native
BackButton drives the return; browser history backs both.

### 5. Reduced motion

`prefers-reduced-motion: reduce` zeroes every animation and transition, removes the
trace's dash so the line is simply present, skips view transitions and the swap
overlays entirely (checked in JS, not just CSS). Someone reading this record may be
unwell; the page is complete and still.

## Rejected

- **Pull-to-refresh** — it fights Telegram's own swipe-down-to-dismiss gesture, and
  this record changes when the owner logs something in chat, not when they tug at
  it. `overscroll-behavior: contain` instead.
- **Count-up numbers** — displays values nobody measured. Hard no.
- **A motion library** — CSS + WAAPI + View Transitions cover every effect here;
  12 kB+ of spring physics earns nothing.
- **Letter-stagger, parallax, ambient loops, skeleton shimmer** — all keep the CPU
  or the eye busy after the page has said what it has to say.
- **Displaying `recent.length`** ("Lihat semua (24)") — even a count is a number
  the server didn't send.
- **Shared-element on the back gesture** — would require re-stamping a name onto a
  list row that may not exist in the new window; a directional fade is honest.

## What the Go side must do

- All strings display-ready: Indonesian decimal comma, thousands dot, **explicit
  minus `−` (U+2212)** on `weight.change`, dates pre-humanised (`"Kam 10 Sep"`,
  Indonesian month/day names). The client never formats.
- `weight.series` coordinates normalised to `0..1`, **y from the top**, with the
  server's own vertical padding baked in. `label` is the full display value
  (`"78,2 kg"`). One point → `x: 0.5`.
- `recent` ordered newest-first; the day view selects by exact `date` string match,
  so the humanised date must be stable within a day.
- `null` for anything unrecorded — never `0`, never `""` for numbers.
- Serve `/api/summary?days=7|30|90` with `content-type: application/json` from the
  same origin as the embedded bundle.
- Optional QA hooks already honoured by the client: `?empty=1` (fresh record),
  `?scheme=light|dark`, `?view=log`.

## Verified

Built clean (`tsc && vite build`, 75.9 kB gzip JS + 3.1 kB CSS); rendered in
headless Chromium at phone width in forced light, forced dark, empty-record and
log-view states (`shots/`). Runs standalone with `npm run dev` against
`src/mock.ts` when the Go server is absent.
