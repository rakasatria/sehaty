import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The build output is embedded into the Sehaty Go binary and served from the
// same origin, so there is no base path and no code splitting games — one
// small bundle, no external requests except Telegram's own SDK script.
export default defineConfig({
  // The bundle is served from /app/ by the Go binary, so asset URLs must be
  // rooted there. Left at "/" the page loads and the assets 404, which shows up
  // as a blank screen rather than an error.
  base: '/app/',
  plugins: [react()],
  build: { target: 'es2020' },
  server: {
    // When the Go server is running locally, /api is proxied to it.
    // When it is not, src/lib/api.ts falls back to src/mock.ts.
    proxy: { '/api': 'http://localhost:8787' },
  },
})
