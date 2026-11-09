const LOCALE = 'es-CO'

const formatters = new Map<string, Intl.NumberFormat>()

function currencyFormat(currency: string): Intl.NumberFormat {
  let f = formatters.get(currency)
  if (!f) {
    f = new Intl.NumberFormat(LOCALE, {
      style: 'currency',
      currency,
      // Guía de Cristal: COP sin decimales
      ...(currency === 'COP' && { maximumFractionDigits: 0 }),
    })
    formatters.set(currency, f)
  }
  return f
}

/** 18.2 → «18,2» (hasta un decimal, con el separador de la configuración regional). */
export function formatPercent(value: string | number): string {
  return new Intl.NumberFormat(LOCALE, { maximumFractionDigits: 1 }).format(Number(value))
}

/** Monto con signo tipográfico (−). `signed` añade + a los positivos. Sin moneda: solo el número. */
export function formatMoney(
  value: string | number,
  currency?: string,
  { signed = false }: { signed?: boolean } = {},
): string {
  const n = Number(value)
  const abs = currency
    ? currencyFormat(currency).format(Math.abs(n))
    : new Intl.NumberFormat(LOCALE).format(Math.abs(n))
  if (n < 0) return `− ${abs}`
  return signed ? `+ ${abs}` : abs
}

/** "USD" -> "US Dollar" (o su equivalente en el idioma del navegador). */
export function currencyLabel(code: string): string {
  const name = new Intl.DisplayNames(LOCALE, { type: 'currency' }).of(code)
  return name ? `${code} · ${name}` : code
}

export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat(LOCALE, { day: 'numeric', month: 'short' })
    .format(new Date(`${iso}T00:00:00`))
    .replace('.', '')
}

/** Primer día del mes como YYYY-MM-DD, en hora local. */
export function monthStart(date = new Date()): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-01`
}

export function todayISO(): string {
  const d = new Date()
  return `${monthStart(d).slice(0, 8)}${String(d.getDate()).padStart(2, '0')}`
}

export function monthLabel(iso: string, style: 'short' | 'long' = 'short'): string {
  return new Intl.DateTimeFormat(LOCALE, { month: style }).format(new Date(`${iso}T00:00:00`))
}
