import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { Glass } from './components'
import { Icon } from './Icon'
import { ToastContext } from './toastContext'

interface ToastItem {
  id: number
  message: string
}

const DURATION_MS = 3800

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const nextId = useRef(1)

  const success = useCallback((message: string) => {
    const id = nextId.current++
    setItems((list) => [...list.slice(-2), { id, message }])
    setTimeout(() => setItems((list) => list.filter((t) => t.id !== id)), DURATION_MS)
  }, [])

  const api = useMemo(() => ({ success }), [success])
  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className="cr cr-toasts" role="status" aria-live="polite">
        {items.map((t) => (
          <Glass key={t.id} strong className="cr-toast">
            <Icon name="check" />
            {t.message}
          </Glass>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
