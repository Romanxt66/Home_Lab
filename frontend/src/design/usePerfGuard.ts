import { useEffect } from 'react'

const WINDOW = 90 // fotogramas con actividad que se evalúan
const SLOW_MS = 40 // un fotograma por encima de esto se nota (≈ <25 fps)
const SLOW_SHARE = 0.35 // si más de esta parte de la ventana va lenta, se pasa al modo ligero
const ACTIVE_MS = 160 // un fotograma cuenta solo si hubo scroll/cursor/toque hace menos de esto

/** ¿Hay que pasar al modo ligero? `deltas` = duración (ms) de los últimos fotogramas con actividad. */
export function shouldGoLite(deltas: number[]): boolean {
  if (deltas.length < WINDOW) return false
  const recent = deltas.slice(-WINDOW)
  return recent.filter((d) => d > SLOW_MS).length / WINDOW > SLOW_SHARE
}

const KEY = 'cr-perf'
const lite = () => {
  document.documentElement.dataset.perf = 'lite'
  try {
    sessionStorage.setItem(KEY, 'lite')
  } catch {
    /* sin sessionStorage: se reevalúa en la próxima carga */
  }
}

/**
 * Red de seguridad de fluidez para el público: el cristal (desenfoque, reflejo, inclinación) es caro en
 * equipos modestos. Si el dispositivo es claramente justo, o se miden fotogramas lentos mientras el usuario
 * hace scroll o mueve el cursor, la página pasa a `data-perf="lite"` (vidrio sólido, sin efectos), que el
 * CSS de glass-fx.css resuelve. Se recuerda durante la sesión. `?lite` / `?full` fuerzan un modo.
 */
export function usePerfGuard() {
  useEffect(() => {
    const root = document.documentElement
    const forced = new URLSearchParams(location.search)
    if (forced.has('full')) return
    let saved: string | null = null
    try {
      saved = sessionStorage.getItem(KEY)
    } catch {
      /* ignorar */
    }
    const nav = navigator as Navigator & { deviceMemory?: number }
    if (
      forced.has('lite') ||
      saved === 'lite' ||
      (nav.deviceMemory ?? 8) <= 2 ||
      (nav.hardwareConcurrency ?? 8) <= 2
    ) {
      lite()
      return
    }
    if (typeof requestAnimationFrame === 'undefined') return

    let lastInput = 0
    const touch = () => {
      lastInput = performance.now()
    }
    const events = ['scroll', 'wheel', 'pointermove', 'touchmove'] as const
    events.forEach((e) => addEventListener(e, touch, { passive: true, capture: true }))

    const deltas: number[] = []
    let raf = 0
    let prev = 0
    const tick = (now: number) => {
      if (prev && !document.hidden && now - lastInput < ACTIVE_MS && now - prev < 500) {
        deltas.push(now - prev)
        if (deltas.length > WINDOW * 2) deltas.splice(0, WINDOW)
        if (shouldGoLite(deltas)) return lite()
      }
      prev = now
      if (root.dataset.perf !== 'lite') raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(raf)
      events.forEach((e) => removeEventListener(e, touch, { capture: true }))
    }
  }, [])
}
