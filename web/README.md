# web — the Mini App

The Telegram Mini App and the browser dashboard are the same React application.
It is built here and **embedded into the Go binary** at
`internal/dashboard/webdist`, so the whole of Sehaty stays one file to copy and
one service to run.

```bash
npm install
npm run dev     # standalone, against src/mock.ts
npm run build   # → dist/
```

To ship a change:

```bash
npm run build && rm -rf ../internal/dashboard/webdist && cp -r dist ../internal/dashboard/webdist
go build ./...  # the embed picks it up
```

`vite.config.ts` sets `base: '/app/'`. Left at `/` the page loads and every asset
404s, which shows up as a blank screen rather than an error.

## The rule this app is built around

**JavaScript may select which value to display. It may never compute one.**

Every figure arrives from `/api/summary` already formatted — Indonesian decimal
comma, thousands dot, U+2212 for the minus, dates humanised. That is not a style
choice: the rules about what may be shown live in Go next to the data, and a
second formatter here would be a second place for a number nobody measured to
appear. There are no count-up animations for the same reason — they display
values on the way to the real one.

`null` means *not recorded*, which is a fact and not a zero, and the interface
draws it as one.

When the server cannot be reached the app says so. It does **not** fall back to
mock data outside `npm run dev`; a plausible wrong number is the single failure
this application exists to prevent.

See `DESIGN.md` for the motion choreography and what was deliberately rejected.
