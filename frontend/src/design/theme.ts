import { useCallback, useEffect, useState } from 'react'

export type ThemeChoice = 'system' | 'light' | 'dark'
const KEY = 'theme'

function read(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : 'system'
  } catch {
    return 'system'
  }
}

export function applyTheme(choice: ThemeChoice) {
  const root = document.documentElement
  if (choice === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', choice)
}

/** Aplica la preferencia guardada antes del primer pintado (evita el parpadeo de tema). */
export function applyStoredTheme() {
  applyTheme(read())
}

const NEXT: Record<ThemeChoice, ThemeChoice> = {
  system: 'light',
  light: 'dark',
  dark: 'system',
}

export function useTheme() {
  const [choice, setChoice] = useState<ThemeChoice>(read)
  useEffect(() => applyTheme(choice), [choice])
  const set = useCallback((next: ThemeChoice) => {
    try {
      if (next === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, next)
    } catch {
      /* preferencia opcional */
    }
    setChoice(next)
  }, [])
  const cycle = useCallback(() => {
    setChoice((c) => {
      const next = NEXT[c]
      try {
        if (next === 'system') localStorage.removeItem(KEY)
        else localStorage.setItem(KEY, next)
      } catch {
        /* preferencia opcional */
      }
      return next
    })
  }, [])
  return { choice, cycle, set }
}
