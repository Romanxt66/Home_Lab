/** Utilidades de fechas del calendario. Todo en la zona del navegador. */

export const DIAS = ["lun", "mar", "mié", "jue", "vie", "sáb", "dom"];

export function pad(n: number): string {
  return String(n).padStart(2, "0");
}

/** «2026-10-27», en la zona local: la clave de un día. */
export function ymd(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export function hhmm(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/**
 * Las 42 celdas (seis semanas) que pinta un mes, empezando en lunes. Se
 * incluyen los días del mes anterior y siguiente para que la rejilla siempre
 * tenga la misma forma y no «salte» de altura al cambiar de mes.
 */
export function celdasDelMes(año: number, mes: number): Date[] {
  const primero = new Date(año, mes, 1);
  // getDay(): domingo = 0. Para empezar en lunes se desplaza.
  const hueco = (primero.getDay() + 6) % 7;

  return Array.from({ length: 42 }, (_, i) => new Date(año, mes, 1 - hueco + i));
}

export function mismoDia(a: Date, b: Date): boolean {
  return ymd(a) === ymd(b);
}

export function nombreDelMes(año: number, mes: number): string {
  return new Date(año, mes, 1).toLocaleDateString("es-CO", { month: "long", year: "numeric" });
}

/** «mar 27 oct»: el encabezado de un día en la agenda. */
export function diaLargo(d: Date): string {
  return d
    .toLocaleDateString("es-CO", { weekday: "short", day: "numeric", month: "short" })
    .replace(/\./g, "");
}

export const RECORDATORIOS: { valor: string; etiqueta: string }[] = [
  { valor: "none", etiqueta: "Sin recordatorio" },
  { valor: "0", etiqueta: "Al comenzar" },
  { valor: "5", etiqueta: "5 minutos antes" },
  { valor: "15", etiqueta: "15 minutos antes" },
  { valor: "30", etiqueta: "30 minutos antes" },
  { valor: "60", etiqueta: "1 hora antes" },
  { valor: "120", etiqueta: "2 horas antes" },
  { valor: "1440", etiqueta: "1 día antes" },
];
