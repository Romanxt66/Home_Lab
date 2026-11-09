import { DONUT_COLORS } from './donutColors'
import {
  Fragment,
  forwardRef,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type CSSProperties,
  type ElementType,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
} from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { formatMoney } from './format'
import { Icon, type IconName } from './Icon'
import { useCountUp } from './useCountUp'
import { usePerfGuard } from './usePerfGuard'

const cx = (...parts: unknown[]) =>
  parts.filter((p): p is string => typeof p === 'string' && p !== '').join(' ')

/** Raíz de tema: aplica tipografía y variables de Cristal a todo lo que contiene. */
export function Backdrop({ children }: { children: ReactNode }) {
  usePerfGuard()
  return <div className="cr cr-backdrop">{children}</div>
}

type GlassProps = HTMLAttributes<HTMLElement> & {
  as?: ElementType
  strong?: boolean
  size?: 'lg' | 'md'
  padded?: boolean
}

export function Glass({ as: Tag = 'div', strong, size, padded, className, ...rest }: GlassProps) {
  return (
    <Tag
      className={cx(
        'cr-glass',
        strong && 'cr-glass-strong',
        size === 'md' && 'cr-glass-md',
        padded && 'cr-pad',
        className,
      )}
      {...rest}
    />
  )
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'glass' | 'ghost' | 'danger'
  size?: 'md' | 'sm'
  icon?: IconName
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'glass', size, icon, className, children, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      className={cx(
        'cr-btn',
        `cr-btn-${variant}`,
        size === 'sm' && 'cr-btn-sm',
        icon && !children && 'cr-btn-icon',
        className,
      )}
      {...rest}
    >
      {icon && <Icon name={icon} />}
      {children}
    </button>
  )
})

export function Badge({
  tone = 'neutral',
  icon,
  children,
}: {
  tone?: 'neutral' | 'positive' | 'negative' | 'warning'
  icon?: IconName
  children: ReactNode
}) {
  return (
    <span className={cx('cr-badge', tone !== 'neutral' && `cr-badge-${tone}`)}>
      {icon && <Icon name={icon} />}
      {children}
    </span>
  )
}

/** Variación porcentual con flecha y signo (no solo color). */
export function Delta({ value, invert }: { value: number; invert?: boolean }) {
  const up = value >= 0
  const good = invert ? !up : up
  return (
    <Badge tone={good ? 'positive' : 'negative'} icon={up ? 'up' : 'down'}>
      {up ? '+' : '−'}
      {Math.abs(value).toLocaleString('es-CO', { maximumFractionDigits: 1 })} %
    </Badge>
  )
}

/**
 * Curva monótona (Fritsch–Carlson): pasa por todos los puntos sin inventar picos ni bajar de cero
 * cuando los datos no lo hacen. Es la misma curva que dibuja la app Android.
 */
export function monotonePath(pts: [number, number][]): string {
  const n = pts.length
  if (n < 2) return ''
  const dx = pts.slice(1).map((p, i) => p[0] - pts[i][0])
  const sl = pts.slice(1).map((p, i) => (p[1] - pts[i][1]) / (dx[i] || 1))
  const m = pts.map((_, i) =>
    i === 0
      ? sl[0]
      : i === n - 1
        ? sl[n - 2]
        : sl[i - 1] * sl[i] <= 0
          ? 0
          : (sl[i - 1] + sl[i]) / 2,
  )
  for (let i = 0; i < n - 1; i++) {
    if (sl[i] === 0) {
      m[i] = m[i + 1] = 0
      continue
    }
    const a = m[i] / sl[i]
    const b = m[i + 1] / sl[i]
    const h = a * a + b * b
    if (h > 9) {
      const t = 3 / Math.sqrt(h)
      m[i] = t * a * sl[i]
      m[i + 1] = t * b * sl[i]
    }
  }
  const f = (v: number) => v.toFixed(1)
  let d = `M${f(pts[0][0])} ${f(pts[0][1])}`
  for (let i = 0; i < n - 1; i++) {
    const [x0, y0] = pts[i]
    const [x1, y1] = pts[i + 1]
    const h = (x1 - x0) / 3
    d += `C${f(x0 + h)} ${f(y0 + m[i] * h)} ${f(x1 - h)} ${f(y1 - m[i + 1] * h)} ${f(x1)} ${f(y1)}`
  }
  return d
}

/** Mini gráfico de tendencia: área con degradado, línea que se dibuja de izquierda a derecha y punto final. */
export function Sparkline({ data, label }: { data: number[]; label: string }) {
  const gid = useId().replace(/:/g, '')
  const w = 120
  const h = 40
  const pad = 3
  if (data.length < 2) return null
  const min = Math.min(...data)
  const span = Math.max(...data) - min || 1
  const pts = data.map(
    (v, i) =>
      [
        pad + (i * (w - pad * 2)) / (data.length - 1),
        pad + (1 - (v - min) / span) * (h - pad * 2),
      ] as [number, number],
  )
  const line = monotonePath(pts)
  const last = pts[pts.length - 1]
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      width={w}
      height={h}
      className="cr-balance-spark"
      role="img"
      aria-label={label}
    >
      <defs>
        <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="currentColor" stopOpacity={0.22} />
          <stop offset="1" stopColor="currentColor" stopOpacity={0} />
        </linearGradient>
      </defs>
      <path
        d={`${line} L${last[0]} ${h} L${pts[0][0]} ${h} Z`}
        fill={`url(#${gid})`}
        className="cr-spark-area"
      />
      <path
        d={line}
        pathLength={1}
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        className="cr-spark-line"
      />
      <circle cx={last[0]} cy={last[1]} r={3} fill="currentColor" className="cr-spark-dot" />
    </svg>
  )
}

/** Escalonado de entrada: pon `style={stagger(i)}` en elementos de lista (el CSS limita el retraso). */
export const stagger = (i: number) => ({ '--i': i }) as CSSProperties

export function BalanceCard(props: {
  value: number
  currency?: string
  label: string
  note?: string
  trend?: number[]
  trendLabel?: string
  hidden?: boolean
  onToggleHidden?: () => void
}) {
  return (
    <Glass strong className="cr-balance">
      <div className="cr-balance-top">
        <span className="overline">{props.label}</span>
        {props.onToggleHidden && (
          <Button
            variant="ghost"
            size="sm"
            icon="eye"
            aria-label={props.hidden ? 'Mostrar saldo' : 'Ocultar saldo'}
            aria-pressed={props.hidden}
            onClick={props.onToggleHidden}
          />
        )}
      </div>
      <div className="amount-xl cr-balance-value">
        {props.hidden ? '••••••' : <AnimatedMoney value={props.value} currency={props.currency} />}
      </div>
      <div className="cr-balance-foot">
        {props.note ? <span className="caption cr-muted">{props.note}</span> : <span />}
        {props.trend && <Sparkline data={props.trend} label={props.trendLabel ?? 'Tendencia'} />}
      </div>
    </Glass>
  )
}

export function StatTile(props: {
  label: string
  value: number
  currency?: string
  delta?: number | null
  invertDelta?: boolean
  icon?: IconName
  tone?: 'brand' | 'amber' | 'negative'
  note?: string
}) {
  return (
    <Glass size="md" className="cr-stat">
      <div className="cr-stat-head">
        <span className={cx('cr-chip', props.tone && `cr-chip-${props.tone}`)}>
          <Icon name={props.icon ?? 'chart'} />
        </span>
        {props.delta != null && <Delta value={props.delta} invert={props.invertDelta} />}
      </div>
      <div>
        <div className="overline">{props.label}</div>
        <div className="amount">
          <AnimatedMoney value={props.value} currency={props.currency} />
        </div>
        {props.note && <div className="caption cr-muted">{props.note}</div>}
      </div>
    </Glass>
  )
}

export type TxKind = 'income' | 'expense' | 'transfer'
const KIND_ICON: Record<TxKind, IconName> = {
  income: 'salary',
  expense: 'card',
  transfer: 'swap',
}
const KIND_TONE: Record<TxKind, string | undefined> = {
  income: 'brand',
  expense: undefined,
  transfer: 'amber',
}

export function TransactionRow(props: {
  kind: TxKind
  title: string
  detail?: string
  amount: number
  currency?: string
  actions?: ReactNode
}) {
  const signed =
    props.kind === 'income' ? props.amount : props.kind === 'expense' ? -props.amount : props.amount
  return (
    <div className="cr-txn" role="listitem">
      <span className={cx('cr-chip', KIND_TONE[props.kind] && `cr-chip-${KIND_TONE[props.kind]}`)}>
        <Icon name={KIND_ICON[props.kind]} />
      </span>
      <div className="cr-txn-main">
        <div className="body cr-txn-title">{props.title}</div>
        {props.detail && <div className="caption cr-muted">{props.detail}</div>}
      </div>
      <div className={cx('amount-sm cr-txn-amt', props.kind === 'income' ? 'cr-pos' : 'cr-neg')}>
        {formatMoney(signed, props.currency, {
          signed: props.kind === 'income',
        })}
      </div>
      {props.actions && <div className="cr-actions-inline">{props.actions}</div>}
    </div>
  )
}

export function TransactionList({ children, label }: { children: ReactNode; label: string }) {
  return (
    <Glass size="md" className="cr-list" role="list" aria-label={label}>
      {children}
    </Glass>
  )
}

export function BudgetBar(props: {
  label: string
  spent: number
  limit: number
  currency?: string
  actions?: ReactNode
}) {
  const ratio = props.limit ? props.spent / props.limit : 0
  const state = ratio > 1 ? 'is-over' : ratio >= 0.8 ? 'is-warn' : ''
  const note =
    ratio > 1
      ? `Excedido en ${formatMoney(props.spent - props.limit, props.currency)}`
      : `Quedan ${formatMoney(props.limit - props.spent, props.currency)}`
  return (
    <Glass size="md" className={cx('cr-budget', state)}>
      <div className="cr-budget-head">
        <span className="body">{props.label}</span>
        <span className="label cr-muted">
          {formatMoney(props.spent, props.currency)} / {formatMoney(props.limit, props.currency)}
        </span>
      </div>
      <div
        className="cr-track"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(Math.min(ratio, 1) * 100)}
        aria-label={props.label}
      >
        <div className="cr-fill" style={{ width: `${Math.min(ratio, 1) * 100}%` }} />
      </div>
      <div className="cr-row-split" style={{ padding: 0 }}>
        <span className="caption cr-muted">
          {Math.round(ratio * 100)} % · {note}
        </span>
        {props.actions}
      </div>
    </Glass>
  )
}

export function Segmented<T extends string>(props: {
  options: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
  label: string
}) {
  return (
    <Glass className="cr-seg" role="group" aria-label={props.label}>
      {props.options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={o.value === props.value}
          onClick={() => props.onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </Glass>
  )
}

export interface NavItem {
  to: string
  label: string
  icon: IconName
  end?: boolean
}

/** Barra de pestañas flotante (móvil). En pantallas anchas se usa la navegación superior. */
/** Barra de pestañas del móvil. `center` (p. ej. el «+» de anotar) va en medio de las pestañas. */
export function TabBar({ items, center }: { items: NavItem[]; center?: ReactNode }) {
  const half = Math.floor(items.length / 2)
  return (
    <Glass as="nav" className="cr-tabbar" aria-label="Navegación principal">
      {items.map((t, i) => (
        <Fragment key={t.to}>
          {center && i === half && center}
          <NavLink to={t.to} end={t.end} className="cr-tab">
            <Icon name={t.icon} />
            {t.label}
          </NavLink>
        </Fragment>
      ))}
    </Glass>
  )
}

export function TopNav({ items }: { items: NavItem[] }) {
  const nav = useRef<HTMLElement>(null)
  const { pathname } = useLocation()

  // Indicador deslizante: una píldora que viaja hasta la pestaña activa (se mide tras cada ruta/redimensión)
  useLayoutEffect(() => {
    const el = nav.current
    if (!el) return
    const place = () => {
      const active = el.querySelector<HTMLElement>('a[aria-current="page"]')
      if (!active) return el.removeAttribute('data-ind')
      el.style.setProperty('--ind-x', `${active.offsetLeft}px`)
      el.style.setProperty('--ind-w', `${active.offsetWidth}px`)
      el.setAttribute('data-ind', '')
    }
    place()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(place)
    ro.observe(el)
    return () => ro.disconnect()
  }, [pathname])

  return (
    <nav ref={nav} className="cr-nav" aria-label="Principal">
      {items.map((t) => (
        <NavLink key={t.to} to={t.to} end={t.end} title={t.label}>
          <Icon name={t.icon} />
          <span className="cr-nav-label">{t.label}</span>
        </NavLink>
      ))}
    </nav>
  )
}

interface FieldProps {
  label: string
  error?: string
}

export const Input = forwardRef<
  HTMLInputElement,
  FieldProps & {
    icon?: IconName
    trailing?: ReactNode
  } & InputHTMLAttributes<HTMLInputElement>
>(({ label, error, className, icon, trailing, ...props }, ref) => {
  const id = useId()
  const input = (
    <input
      id={id}
      ref={ref}
      className={cx('cr-input', icon && 'cr-input-icon', trailing && 'cr-input-trail', className)}
      aria-invalid={!!error}
      aria-describedby={error ? `${id}-error` : undefined}
      {...props}
    />
  )
  return (
    <div className="cr-field">
      <label htmlFor={id}>{label}</label>
      {icon || trailing ? (
        <div className="cr-input-wrap">
          {icon && (
            <span className="cr-input-lead" aria-hidden="true">
              <Icon name={icon} />
            </span>
          )}
          {input}
          {trailing && <span className="cr-input-end">{trailing}</span>}
        </div>
      ) : (
        input
      )}
      {error && (
        <p className="cr-error" id={`${id}-error`}>
          {error}
        </p>
      )}
    </div>
  )
})

/** Contraseña con candado y botón para mostrarla u ocultarla. */
export const PasswordInput = forwardRef<
  HTMLInputElement,
  FieldProps & Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>
>((props, ref) => {
  const [visible, setVisible] = useState(false)
  return (
    <Input
      {...props}
      ref={ref}
      type={visible ? 'text' : 'password'}
      icon="lock"
      trailing={
        <button
          type="button"
          className="cr-input-toggle"
          aria-label={visible ? 'Ocultar contraseña' : 'Mostrar contraseña'}
          aria-pressed={visible}
          onClick={() => setVisible((v) => !v)}
        >
          <Icon name={visible ? 'eyeOff' : 'eye'} />
        </button>
      }
    />
  )
})

export const Checkbox = forwardRef<
  HTMLInputElement,
  { label: ReactNode } & Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>
>(({ label, className, ...props }, ref) => (
  <label className={cx('cr-check', className)}>
    <input ref={ref} type="checkbox" {...props} />
    <span className="cr-check-box" aria-hidden="true">
      <Icon name="check" />
    </span>
    <span>{label}</span>
  </label>
))

export function Card({
  title,
  action,
  children,
  flush,
}: {
  title?: string
  action?: ReactNode
  children: ReactNode
  flush?: boolean
}) {
  return (
    <Glass as="section" padded={!flush} className={flush ? 'cr-bleed' : undefined}>
      {(title || action) && (
        <div className={cx('cr-card-title', flush && 'cr-row-split')}>
          <h2 className="heading">{title}</h2>
          {action}
        </div>
      )}
      {children}
    </Glass>
  )
}

export function ErrorText({ error }: { error: unknown }) {
  if (!error) return null
  return (
    <p role="alert" className="cr-error">
      {error instanceof Error ? error.message : 'Error inesperado'}
    </p>
  )
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="cr-empty">{children}</p>
}

export function Skeleton({ lines = 3, height = 56 }: { lines?: number; height?: number }) {
  return (
    <div className="cr-skeleton-stack" role="status">
      <span className="cr-sr">Cargando…</span>
      {Array.from({ length: lines }, (_, i) => (
        <span key={i} className="cr-skeleton" style={{ height }} aria-hidden="true" />
      ))}
    </div>
  )
}

/** Cifra que cuenta hasta su valor al aparecer o cambiar. */
export function AnimatedMoney({ value, currency }: { value: number; currency?: string }) {
  return <>{formatMoney(useCountUp(value), currency)}</>
}

/** Tarjeta de progreso positivo (metas de ahorro): llena hacia el objetivo; al lograrlo se destaca. */
export function ProgressCard(props: {
  label: string
  saved: number
  target: number
  currency?: string
  achieved?: boolean
  note?: string
  actions?: ReactNode
}) {
  const ratio = props.target ? props.saved / props.target : 0
  return (
    <Glass size="md" className={cx('cr-budget', props.achieved && 'cr-goal-done')}>
      <div className="cr-budget-head">
        <span className="body">{props.label}</span>
        {props.achieved ? (
          <Badge tone="positive" icon="check">
            Lograda
          </Badge>
        ) : (
          <span className="label cr-muted">{Math.round(Math.min(ratio, 1) * 100)} %</span>
        )}
      </div>
      <div className="amount">
        <AnimatedMoney value={props.saved} currency={props.currency} />
        <span className="label cr-muted"> de {formatMoney(props.target, props.currency)}</span>
      </div>
      <div
        className="cr-track"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(Math.min(ratio, 1) * 100)}
        aria-label={props.label}
      >
        <div className="cr-fill" style={{ width: `${Math.min(ratio, 1) * 100}%` }} />
      </div>
      {props.note && <span className="caption cr-muted">{props.note}</span>}
      {props.actions && <div className="cr-goal-actions">{props.actions}</div>}
    </Glass>
  )
}

/** Dona de participación. El color no es la única pista: el significado va en la leyenda (lista) contigua. */
export function Donut({
  data,
  label,
  size = 132,
}: {
  data: { key: string; value: number }[]
  label: string
  size?: number
}) {
  const total = data.reduce((a, d) => a + d.value, 0)
  const radius = 15.9155 // circunferencia = 100: los porcentajes caen directo en stroke-dasharray
  let offset = 25 // arranca arriba
  return (
    <svg
      viewBox="0 0 42 42"
      width={size}
      height={size}
      role="img"
      aria-label={label}
      className="cr-donut"
    >
      <circle cx="21" cy="21" r={radius} fill="none" stroke="var(--hairline)" strokeWidth="5" />
      {total > 0 &&
        data.map((d, i) => {
          const pct = (d.value / total) * 100
          const el = (
            <circle
              key={d.key}
              cx="21"
              cy="21"
              r={radius}
              fill="none"
              stroke={DONUT_COLORS[i % DONUT_COLORS.length]}
              strokeOpacity={i < DONUT_COLORS.length ? 1 : 0.55}
              strokeWidth="5"
              strokeDasharray={`${Math.max(pct - 0.6, 0.01)} ${100 - Math.max(pct - 0.6, 0.01)}`}
              strokeDashoffset={offset}
              className="cr-donut-seg"
            />
          )
          offset -= pct
          return el
        })}
    </svg>
  )
}

/** Logotipo de Alvea (la «A» plateada con la línea de tendencia). */
export function BrandLogo({ size = 34 }: { size?: number }) {
  return (
    <img
      className="cr-brand-logo"
      src={size > 64 ? '/brand/logo-256.png' : '/brand/logo-128.png'}
      width={size}
      height={size}
      alt=""
      decoding="async"
    />
  )
}

/**
 * Alta plegable: sin datos se muestra abierta (es lo primero que hay que hacer); con datos queda tras
 * un botón con el mismo título, para que la lista sea lo primero. Una vez abierta, se queda abierta.
 */
export function CreateSection({
  title,
  hasData,
  children,
}: {
  title: string
  hasData: boolean
  children: ReactNode
}) {
  const [open, setOpen] = useState<boolean | null>(null)
  const shown = open ?? !hasData
  if (!shown) {
    return (
      <div>
        <Button variant="primary" icon="plus" onClick={() => setOpen(true)}>
          {title}
        </Button>
      </div>
    )
  }
  return (
    // Al empezar a escribir queda abierta aunque lleguen datos después (no se pliega bajo los dedos)
    <div onFocusCapture={() => open === null && setOpen(true)}>
      <Card
        title={title}
        action={
          hasData ? (
            <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>
              Cerrar
            </Button>
          ) : undefined
        }
      >
        {children}
      </Card>
    </div>
  )
}

/**
 * Encabezado de página como en la app Android: etiqueta pequeña con el nombre y un titular con una
 * parte en serif cursiva (`*así*`). El nombre accesible del h1 es el título corto («Movimientos»).
 */
export function PageTitle({ title, headline }: { title: string; headline: string }) {
  const parts = headline.split('*')
  return (
    <div className="cr-page-title">
      <span className="cr-page-tag" aria-hidden="true">
        {title.toLowerCase()}
      </span>
      <h1 className="cr-headline" aria-label={title}>
        {parts.map((part, i) => (i % 2 ? <em key={i}>{part}</em> : <span key={i}>{part}</span>))}
      </h1>
    </div>
  )
}

/** Selector de mes en píldora (‹ noviembre 2026 ›), como en la app. `value` es yyyy-MM-01. */
export function MonthPill({
  value,
  onChange,
}: {
  value: string
  onChange: (month: string) => void
}) {
  const [y, m] = value.split('-').map(Number)
  const shift = (n: number) => {
    const d = new Date(y, m - 1 + n, 1)
    onChange(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`)
  }
  const label = new Date(y, m - 1, 1).toLocaleDateString('es-CO', {
    month: 'long',
    year: 'numeric',
  })
  return (
    <Glass className="cr-month-pill" role="group" aria-label="Mes">
      <button type="button" aria-label="Mes anterior" onClick={() => shift(-1)}>
        <Icon name="chevron" />
      </button>
      <span aria-live="polite">{label.replace(' de ', ' ')}</span>
      <button type="button" aria-label="Mes siguiente" onClick={() => shift(1)}>
        <Icon name="chevron" />
      </button>
    </Glass>
  )
}
