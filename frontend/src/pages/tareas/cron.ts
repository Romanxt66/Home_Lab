/**
 * Cron de cinco campos —minuto hora día mes día-de-la-semana—, igual que el
 * del servidor: «*», pasos «/n», rangos «a-b», «a-b/n» y listas con comas. El
 * domingo vale 0 o 7.
 *
 * Esto solo da respuesta inmediata mientras se escribe. Quien decide si una
 * expresión vale es el servidor, que la vuelve a comprobar al guardar.
 */

const RANGOS: [number, number][] = [
  [0, 59],
  [0, 23],
  [1, 31],
  [1, 12],
  [0, 7],
];
const NOMBRES = ["minuto", "hora", "día", "mes", "día de la semana"];

function campoValido(campo: string, [min, max]: [number, number]): boolean {
  return campo.split(",").every((parte) => {
    const [cuerpo, paso, ...resto] = parte.split("/");
    if (resto.length > 0) return false;

    if (paso !== undefined && !/^\d+$/.test(paso)) return false;
    if (paso !== undefined && Number(paso) <= 0) return false;

    if (cuerpo === "*") return true;

    const rango = cuerpo.split("-");
    if (rango.length > 2 || !rango.every((n) => /^\d+$/.test(n))) return false;

    const [desde, hasta = desde] = rango.map(Number);
    return desde >= min && hasta <= max && desde <= hasta;
  });
}

/** null si la expresión vale; si no, qué le pasa, en una frase. */
export function errorDeCron(expr: string): string | null {
  const campos = expr.trim().split(/\s+/).filter(Boolean);

  if (campos.length !== 5) {
    return "Un cron tiene 5 campos: minuto hora día mes día-semana (por ejemplo */5 * * * *).";
  }

  for (let i = 0; i < 5; i++) {
    if (!campoValido(campos[i], RANGOS[i])) {
      return `El campo «${NOMBRES[i]}» no es válido: ${campos[i]}`;
    }
  }

  return null;
}

const DIAS = ["domingos", "lunes", "martes", "miércoles", "jueves", "viernes", "sábados", "domingos"];

const esNumero = (s: string) => /^\d+$/.test(s);
const hora = (h: string, m: string) => `${h.padStart(2, "0")}:${m.padStart(2, "0")}`;

/** Una frase para las expresiones habituales; el cron tal cual para el resto. */
export function describirCron(expr: string): string {
  const [m, h, d, mes, sem] = expr.trim().split(/\s+/);

  if (errorDeCron(expr) !== null) return "";
  const todosLosMeses = mes === "*";

  if (m === "*" && h === "*" && d === "*" && todosLosMeses && sem === "*") return "cada minuto";

  const pasoMin = /^\*\/(\d+)$/.exec(m);
  if (pasoMin && h === "*" && d === "*" && todosLosMeses && sem === "*") return `cada ${pasoMin[1]} minutos`;

  if (h === "*" && d === "*" && todosLosMeses && sem === "*" && esNumero(m)) {
    return m === "0" ? "cada hora, en punto" : `cada hora, en el minuto ${m}`;
  }

  const pasoHora = /^\*\/(\d+)$/.exec(h);
  if (pasoHora && m === "0" && d === "*" && todosLosMeses && sem === "*") return `cada ${pasoHora[1]} horas`;

  if (esNumero(m) && esNumero(h) && todosLosMeses) {
    const a = hora(h, m);

    if (d === "*" && sem === "*") return `todos los días a las ${a}`;
    if (d === "*" && sem === "1-5") return `de lunes a viernes a las ${a}`;
    if (d === "*" && esNumero(sem)) return `los ${DIAS[Number(sem)]} a las ${a}`;
    if (esNumero(d) && sem === "*") return `el día ${d} de cada mes a las ${a}`;
  }

  return `según el cron «${expr.trim()}»`;
}

export const PRESETS: { etiqueta: string; cron: string }[] = [
  { etiqueta: "Cada 15 minutos", cron: "*/15 * * * *" },
  { etiqueta: "Cada hora", cron: "0 * * * *" },
  { etiqueta: "Cada día a las 9", cron: "0 9 * * *" },
  { etiqueta: "Lunes a viernes a las 9", cron: "0 9 * * 1-5" },
  { etiqueta: "Domingos a las 10", cron: "0 10 * * 0" },
];
