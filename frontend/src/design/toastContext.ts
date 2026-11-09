import { createContext, useContext } from 'react'

export interface ToastApi {
  success: (message: string) => void
}

// Sin proveedor (tests, vistas aisladas) los avisos simplemente no se muestran.
export const ToastContext = createContext<ToastApi>({ success: () => {} })

export const useToast = () => useContext(ToastContext)
