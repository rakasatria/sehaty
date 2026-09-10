import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/tokens.css'
import './styles/app.css'
import { initTelegram } from './lib/telegram'
import App from './App'

// QA hook: ?scheme=light|dark pins the fallback palette in a plain browser.
const forcedScheme = new URLSearchParams(location.search).get('scheme')
if (forcedScheme === 'light' || forcedScheme === 'dark') {
  document.documentElement.dataset.forcedScheme = forcedScheme
  document.documentElement.style.colorScheme = forcedScheme
}

initTelegram()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
