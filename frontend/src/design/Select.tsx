import {
  Children,
  forwardRef,
  isValidElement,
  useCallback,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type SelectHTMLAttributes,
} from 'react'
import { createPortal } from 'react-dom'
import { Icon } from './Icon'

interface Option {
  value: string
  label: string
  disabled: boolean
}

const MENU_MAX_HEIGHT = 288
const MENU_GAP = 6

const cx = (...parts: (string | false | null | undefined)[]) => parts.filter(Boolean).join(' ')

function textOf(node: ReactNode): string {
  if (node == null || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(textOf).join('')
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children)
  return ''
}

/** Extrae las <option> (también dentro de fragmentos y de <optgroup>) de los hijos del select. */
function parseOptions(children: ReactNode): Option[] {
  const out: Option[] = []
  const walk = (nodes: ReactNode) => {
    Children.forEach(nodes, (child) => {
      if (
        !isValidElement<{
          value?: string | number
          disabled?: boolean
          children?: ReactNode
        }>(child)
      )
        return
      if (child.type === 'option') {
        const label = textOf(child.props.children)
        out.push({
          value: child.props.value !== undefined ? String(child.props.value) : label,
          label,
          disabled: !!child.props.disabled,
        })
      } else if (child.props.children) {
        walk(child.props.children)
      }
    })
  }
  walk(children)
  return out
}

const finePointer = () =>
  typeof window.matchMedia === 'function' && window.matchMedia('(pointer: fine)').matches

interface Placement {
  top: number
  left: number
  width: number
  maxHeight: number
  up: boolean
}

/**
 * Desplegable de Cristal. El <select> nativo sigue siendo el control accesible y el que
 * recibe el valor (formularios, lectores de pantalla y selector del sistema en móvil);
 * con puntero fino y con teclado se sustituye su lista por un menú de vidrio animado.
 */
export const Select = forwardRef<
  HTMLSelectElement,
  { label: string; error?: string } & SelectHTMLAttributes<HTMLSelectElement>
>(function Select({ label, error, className, children, onKeyDown, onBlur, ...props }, ref) {
  const id = useId()
  const listId = `${id}-list`
  const field = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLUListElement>(null)
  const native = useRef<HTMLSelectElement | null>(null)
  const typed = useRef({ text: '', at: 0 })
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const [value, setValue] = useState('')
  const [placement, setPlacement] = useState<Placement | null>(null)
  const options = parseOptions(children)

  const setRefs = useCallback(
    (el: HTMLSelectElement | null) => {
      native.current = el
      if (typeof ref === 'function') ref(el)
      else if (ref) ref.current = el
    },
    [ref],
  )

  // El valor vive en el DOM (react-hook-form lo cambia con reset()): se refleja tras cada render
  // oxlint-disable-next-line react-hooks/exhaustive-deps -- debe correr tras cada render (ver arriba)
  useLayoutEffect(() => {
    const current = native.current?.value ?? ''
    if (current !== value) setValue(current)
  })

  const measure = useCallback(() => {
    const box = field.current?.querySelector('.cr-select-face')?.getBoundingClientRect()
    if (!box) return
    const below = window.innerHeight - box.bottom - MENU_GAP - 12
    const above = box.top - MENU_GAP - 12
    const up = below < 180 && above > below
    const maxHeight = Math.max(120, Math.min(MENU_MAX_HEIGHT, up ? above : below))
    setPlacement({
      left: box.left,
      width: box.width,
      maxHeight,
      up,
      top: up ? box.top - MENU_GAP : box.bottom + MENU_GAP,
    })
  }, [])

  useLayoutEffect(() => {
    if (!open) return
    measure()
    // El menú tiene su propio scroll: solo cierra el de la página (o el de otro contenedor). Y solo si
    // el campo se movió de verdad: un reajuste mínimo (datos que llegan y mueven la página un píxel)
    // también lanza «scroll» y cerraba el menú recién abierto.
    const top = () =>
      field.current?.querySelector('.cr-select-face')?.getBoundingClientRect().top ?? 0
    const openedAt = top()
    const close = (event: Event) => {
      if (event.target instanceof Node && menuRef.current?.contains(event.target)) return
      if (Math.abs(top() - openedAt) < 8) return
      setOpen(false)
    }
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', close, true)
    return () => {
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', close, true)
    }
  }, [open, measure])

  const selectedIndex = options.findIndex((o) => o.value === value)
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined

  const show = () => {
    if (native.current?.disabled || options.length === 0) return
    setActive(
      selectedIndex >= 0
        ? selectedIndex
        : Math.max(
            0,
            options.findIndex((o) => !o.disabled),
          ),
    )
    setOpen(true)
  }

  const choose = (index: number) => {
    const option = options[index]
    const el = native.current
    setOpen(false)
    if (!option || option.disabled || !el || el.value === option.value) return
    el.value = option.value
    el.dispatchEvent(new Event('change', { bubbles: true }))
    setValue(option.value)
  }

  const step = (from: number, dir: 1 | -1) => {
    for (let i = from + dir; i >= 0 && i < options.length; i += dir) {
      if (!options[i].disabled) return i
    }
    return from
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLSelectElement>) => {
    onKeyDown?.(event)
    if (event.defaultPrevented || (event.altKey && !open && event.key !== 'ArrowDown')) return
    const { key } = event
    if (!open) {
      if (['Enter', ' ', 'ArrowDown', 'ArrowUp'].includes(key) && finePointer()) {
        event.preventDefault()
        show()
      }
      return
    }
    if (key === 'Escape' || key === 'Tab') {
      if (key === 'Escape') event.preventDefault()
      setOpen(false)
    } else if (key === 'ArrowDown') {
      event.preventDefault()
      setActive((i) => step(i, 1))
    } else if (key === 'ArrowUp') {
      event.preventDefault()
      setActive((i) => step(i, -1))
    } else if (key === 'Home' || key === 'End') {
      event.preventDefault()
      const edge = key === 'Home' ? -1 : options.length
      setActive(step(edge, key === 'Home' ? 1 : -1))
    } else if (key === 'Enter' || key === ' ') {
      event.preventDefault()
      choose(active)
    } else if (key.length === 1 && !event.ctrlKey && !event.metaKey) {
      const now = event.timeStamp
      const text = (now - typed.current.at > 700 ? '' : typed.current.text) + key.toLowerCase()
      typed.current = { text, at: now }
      const hit = options.findIndex((o) => !o.disabled && o.label.toLowerCase().startsWith(text))
      if (hit >= 0) setActive(hit)
    }
  }

  const handleMouseDown = (event: MouseEvent<HTMLSelectElement>) => {
    if (!finePointer() || native.current?.disabled) return
    event.preventDefault()
    native.current?.focus()
    if (open) setOpen(false)
    else show()
  }

  const menu =
    open && placement
      ? createPortal(
          <ul
            ref={menuRef}
            id={listId}
            role="listbox"
            aria-label={label}
            className={cx('cr cr-menu', placement.up && 'cr-menu-up')}
            // Sin esto, arrastrar la barra de desplazamiento quita el foco del campo y cierra el menú
            onMouseDown={(e) => e.preventDefault()}
            style={{
              left: placement.left,
              width: placement.width,
              maxHeight: placement.maxHeight,
              ...(placement.up
                ? { bottom: window.innerHeight - placement.top }
                : { top: placement.top }),
            }}
          >
            {options.map((option, index) => (
              <li
                key={`${option.value}-${index}`}
                id={`${id}-opt-${index}`}
                role="option"
                aria-selected={index === selectedIndex}
                aria-disabled={option.disabled || undefined}
                data-active={index === active || undefined}
                className="cr-menu-item"
                style={{ animationDelay: `${Math.min(index, 8) * 18}ms` }}
                ref={(el) => {
                  if (el && index === active) el.scrollIntoView?.({ block: 'nearest' })
                }}
                onMouseDown={(e) => {
                  e.preventDefault()
                  choose(index)
                }}
                onMouseMove={() => !option.disabled && setActive(index)}
              >
                <span>{option.label || ' '}</span>
                {index === selectedIndex && <Icon name="check" />}
              </li>
            ))}
          </ul>,
          document.body,
        )
      : null

  return (
    <div className="cr-field" ref={field}>
      <label htmlFor={id}>{label}</label>
      <div className="cr-select" data-open={open || undefined}>
        <div
          className={cx('cr-input cr-select-face', className)}
          aria-hidden="true"
          data-placeholder={selected?.value === '' || !selected || undefined}
          data-invalid={!!error || undefined}
        >
          <span className="cr-select-value">{selected?.label || ' '}</span>
          <svg viewBox="0 0 24 24" className="cr-select-chevron">
            <path d="M6 9l6 6 6-6" />
          </svg>
        </div>
        <select
          {...props}
          id={id}
          ref={setRefs}
          className="cr-select-native"
          aria-invalid={!!error}
          aria-expanded={open}
          aria-controls={open ? listId : undefined}
          aria-activedescendant={open ? `${id}-opt-${active}` : undefined}
          onKeyDown={handleKeyDown}
          onMouseDown={handleMouseDown}
          onBlur={(event) => {
            setOpen(false)
            onBlur?.(event)
          }}
        >
          {children}
        </select>
      </div>
      {error && <p className="cr-error">{error}</p>}
      {menu}
    </div>
  )
})
