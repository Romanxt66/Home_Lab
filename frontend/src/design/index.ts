// Kit Cristal (Alvea) para React. cristal.css se importa una sola vez, en main.tsx.
// Se omiten Alvi y las caras de ánimo: son la mascota de Alvea y aquí no pintan nada.
export * from './components'
export { Icon, type IconName } from './Icon'
export { ToastProvider } from './Toast'
export { useToast } from './toastContext'
export { ConfirmDialog } from './Confirm'
export { Select } from './Select'
export { applyStoredTheme, useTheme, type ThemeChoice } from './theme'
export { useCountUp } from './useCountUp'
export { usePerfGuard, shouldGoLite } from './usePerfGuard'
export { DONUT_COLORS } from './donutColors'
export * from './format'
