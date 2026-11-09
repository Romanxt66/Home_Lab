import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './design/cristal.css'
import './app.css'
import { Backdrop, ToastProvider, applyStoredTheme } from './design'
import App from './App.tsx'

// Antes del primer pintado: así no hay parpadeo si el tema guardado es el claro.
applyStoredTheme()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Backdrop>
      <ToastProvider>
        <App />
      </ToastProvider>
    </Backdrop>
  </StrictMode>,
)
