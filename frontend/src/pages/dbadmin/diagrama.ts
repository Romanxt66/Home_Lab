import type { DiagramaDb } from "../../api/tipos";

/**
 * Geometría del diagrama de relaciones. Es cálculo puro, sin DOM, para que el
 * componente solo dibuje lo que esto devuelve. Es la del Lab, sin cambios.
 */

export const ANCHO_NODO = 220;
export const ALTO_CABECERA = 30;
export const ALTO_FILA = 18;
export const MAX_COLUMNAS = 10;
const HUECO_H = 90;
const HUECO_V = 60;
const MARGEN = 40;

export interface ColumnaDibujo {
  name: string;
  dataType: string;
  isPrimaryKey: boolean;
  isForeignKey: boolean;
  /** Centro vertical de la fila, relativo a la parte superior del nodo. */
  offsetY: number;
}

export interface NodoDibujo {
  table: string;
  x: number;
  y: number;
  width: number;
  height: number;
  columns: ColumnaDibujo[];
  /** Columnas que no caben porque la tabla pasa de MAX_COLUMNAS. */
  hiddenCount: number;
}

export interface AristaDibujo {
  id: string;
  from: string;
  to: string;
  label: string;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface Dibujo {
  nodes: NodoDibujo[];
  edges: AristaDibujo[];
  width: number;
  height: number;
}

const altoNodo = (visibles: number, hayOcultas: boolean) =>
  ALTO_CABECERA + (visibles + (hayOcultas ? 1 : 0)) * ALTO_FILA + 8;

/**
 * Coloca las tablas en una rejilla, las más conectadas primero para que las
 * más importantes queden arriba a la izquierda. Las flechas salen de las filas
 * concretas de la clave ajena cuando esas filas se ven.
 */
export function disponer(diagrama: DiagramaDb): Dibujo {
  const grado = new Map<string, number>();
  const sumar = (t: string) => grado.set(t, (grado.get(t) ?? 0) + 1);
  for (const r of diagrama.relations) {
    sumar(r.table);
    sumar(r.refTable);
  }

  const ordenadas = [...diagrama.tables].sort((a, b) => {
    const d = (grado.get(b.name) ?? 0) - (grado.get(a.name) ?? 0);
    return d !== 0 ? d : a.name.localeCompare(b.name);
  });

  const porFila = Math.max(1, Math.ceil(Math.sqrt(ordenadas.length || 1)));
  const nodes: NodoDibujo[] = [];
  const altosDeFila: number[] = [];

  ordenadas.forEach((t, i) => {
    const fila = Math.floor(i / porFila);
    const col = i % porFila;
    const visibles = t.columns.slice(0, MAX_COLUMNAS);
    const hiddenCount = Math.max(0, t.columns.length - visibles.length);
    const height = altoNodo(visibles.length, hiddenCount > 0);
    altosDeFila[fila] = Math.max(altosDeFila[fila] ?? 0, height);

    nodes.push({
      table: t.name,
      x: MARGEN + col * (ANCHO_NODO + HUECO_H),
      y: 0, // se resuelve abajo, cuando se sabe lo que mide la tabla más alta de cada fila
      width: ANCHO_NODO,
      height,
      hiddenCount,
      columns: visibles.map((c, idx) => ({
        name: c.name,
        dataType: c.dataType,
        isPrimaryKey: c.isPrimaryKey,
        isForeignKey: c.isForeignKey,
        offsetY: ALTO_CABECERA + idx * ALTO_FILA + ALTO_FILA / 2,
      })),
    });
  });

  const cimas: number[] = [];
  let cursor = MARGEN;
  altosDeFila.forEach((h, r) => {
    cimas[r] = cursor;
    cursor += h + HUECO_V;
  });
  nodes.forEach((n, i) => {
    n.y = cimas[Math.floor(i / porFila)] ?? MARGEN;
  });

  const porTabla = new Map(nodes.map((n) => [n.table, n]));

  const edges: AristaDibujo[] = [];
  diagrama.relations.forEach((rel, i) => {
    const desde = porTabla.get(rel.table);
    const hasta = porTabla.get(rel.refTable);
    // Una relación que apunta fuera del esquema dibujado no se pinta.
    if (!desde || !hasta) return;

    const colDesde = desde.columns.find((c) => c.name === rel.columns[0]);
    const colHasta = hasta.columns.find((c) => c.name === rel.refColumns[0]);
    const y1 = desde.y + (colDesde?.offsetY ?? desde.height / 2);
    const y2 = hasta.y + (colHasta?.offsetY ?? hasta.height / 2);

    // Sale por el lado que mira al destino, para que la línea no atraviese la caja.
    const haciaDerecha = desde.x + desde.width / 2 <= hasta.x + hasta.width / 2;
    const x1 = haciaDerecha ? desde.x + desde.width : desde.x;
    const x2 = haciaDerecha ? hasta.x : hasta.x + hasta.width;

    edges.push({
      id: `${rel.constraintName}-${i}`,
      from: rel.table,
      to: rel.refTable,
      label: `${rel.columns.join(", ")} → ${rel.refColumns.join(", ")}`,
      x1, y1, x2, y2,
    });
  });

  return {
    nodes,
    edges,
    width: nodes.reduce((m, n) => Math.max(m, n.x + n.width), 0) + MARGEN,
    height: nodes.reduce((m, n) => Math.max(m, n.y + n.height), 0) + MARGEN,
  };
}

/** Curva de Bézier entre dos anclajes, abombada en horizontal. */
export function trazo(e: AristaDibujo): string {
  const dx = Math.max(40, Math.abs(e.x2 - e.x1) / 2);
  const c1 = e.x1 + (e.x2 >= e.x1 ? dx : -dx);
  const c2 = e.x2 - (e.x2 >= e.x1 ? dx : -dx);
  return `M ${e.x1} ${e.y1} C ${c1} ${e.y1}, ${c2} ${e.y2}, ${e.x2} ${e.y2}`;
}
