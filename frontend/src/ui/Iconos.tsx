import type { CSSProperties } from "react";

/** Trazos en una caja de 24×24. Para uno nuevo, basta con añadirlo aquí. */
const TRAZOS = {
  pulse: "M3 12h4l2.5-6 4 12L16 12h5",
  plus: "M12 5v14M5 12h14",
  refresh: "M20 11a8 8 0 0 0-14.5-3.5M4 4v4h4M4 13a8 8 0 0 0 14.5 3.5M20 20v-4h-4",
  edit: "M4 20h4L19 9a2.1 2.1 0 0 0-3-3L5 17v3zM14 8l3 3",
  trash: "M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3",
  logout: "M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 8l-4 4 4 4M6 12h10",
  pause: "M8 5v14M16 5v14",
  check: "M5 12.5l4.5 4.5L19 7.5",
  alert: "M12 4l9 16H3L12 4zM12 10v4M12 17v.5",
  globe: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18",
  clock: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 7v5l3 2",
  list: "M4 6h16M4 12h16M4 18h16",
  close: "M6 6l12 12M18 6L6 18",
  user: "M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0",
  info: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 11v5M12 8v.5",
} as const;

export type NombreIcono = keyof typeof TRAZOS;

interface Props {
  nombre: NombreIcono;
  tamano?: number;
  style?: CSSProperties;
}

/**
 * Icono de trazo. Decorativo por defecto (aria-hidden): el botón o enlace que
 * lo contiene es quien lleva la etiqueta accesible.
 */
export function Icono({ nombre, tamano = 18, style }: Props) {
  return (
    <svg
      width={tamano}
      height={tamano}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      style={{ flex: "none", ...style }}
    >
      <path d={TRAZOS[nombre]} />
    </svg>
  );
}
