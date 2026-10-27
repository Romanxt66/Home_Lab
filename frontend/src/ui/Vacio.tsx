import type { ReactNode } from "react";

import { Icono } from "./Iconos";
import type { NombreIcono } from "./Iconos";

interface Props {
  icono: NombreIcono;
  /** Titular. Lo que vaya entre asteriscos sale en serif cursiva. */
  titular: string;
  children?: ReactNode;
  accion?: ReactNode;
}

/** Estado vacío: dice qué falta y cómo conseguirlo, sin culpar a nadie. */
export function Vacio({ icono, titular, children, accion }: Props) {
  return (
    <div className="cristal vacio subir">
      <span className="vacio-icono">
        <Icono nombre={icono} tamano={24} />
      </span>
      <h2>{titular.split("*").map((t, i) => (i % 2 === 1 ? <em key={i}>{t}</em> : t))}</h2>
      {children && <p>{children}</p>}
      {accion}
    </div>
  );
}

/** Tarjetas de cristal con brillo, mientras llega lo que se pidió. */
export function Esqueletos({ cantidad = 3 }: { cantidad?: number }) {
  return (
    <div className="rejilla" aria-busy="true" aria-label="Cargando">
      {Array.from({ length: cantidad }, (_, i) => (
        <div key={i} className="cristal esqueleto" />
      ))}
    </div>
  );
}
