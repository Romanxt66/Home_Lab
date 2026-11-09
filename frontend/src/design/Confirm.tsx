import { useEffect, useId, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button, Glass } from './components'

/** Diálogo modal de confirmación: Escape cancela, el foco entra al botón seguro y vuelve al cerrar. */
export function ConfirmDialog(props: {
  open: boolean
  title: string
  children?: ReactNode
  confirmLabel?: string
  onConfirm: () => void
  onCancel: () => void
}) {
  const titleId = useId()
  const { open, onCancel } = props
  const cancelRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    cancelRef.current?.focus()
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onCancel()
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      previous?.focus?.()
    }
  }, [open, onCancel])

  if (!props.open) return null
  // En el <body>: así ningún contexto de apilamiento de la página (animaciones) lo tapa
  return createPortal(
    <div
      className="cr cr-overlay"
      onMouseDown={(e) => e.target === e.currentTarget && props.onCancel()}
    >
      <Glass
        strong
        padded
        className="cr-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <h2 id={titleId} className="heading">
          {props.title}
        </h2>
        {props.children && <p className="body cr-muted">{props.children}</p>}
        <div className="cr-dialog-actions">
          <Button ref={cancelRef} variant="ghost" onClick={props.onCancel}>
            Cancelar
          </Button>
          <Button variant="danger" onClick={props.onConfirm}>
            {props.confirmLabel ?? 'Eliminar'}
          </Button>
        </div>
      </Glass>
    </div>,
    document.body,
  )
}
