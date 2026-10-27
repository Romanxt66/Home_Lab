import type { ReactNode } from "react";

import { Icono } from "./Iconos";

interface Props {
  tipo?: "error" | "info";
  children: ReactNode;
  alCerrar?(): void;
}

/** Aviso en línea. Los errores se anuncian a los lectores de pantalla; lo demás, con calma. */
export function Alerta({ tipo = "info", children, alCerrar }: Props) {
  return (
    <div
      className={`alerta ${tipo === "error" ? "alerta-error" : "alerta-info"} subir`}
      role={tipo === "error" ? "alert" : "status"}
    >
      <Icono nombre={tipo === "error" ? "alert" : "check"} tamano={16} />
      <span style={{ flex: 1, minWidth: 0, overflowWrap: "anywhere" }}>{children}</span>
      {alCerrar && (
        <button type="button" className="accion" onClick={alCerrar} aria-label="Descartar aviso" title="Descartar">
          <Icono nombre="close" tamano={14} />
        </button>
      )}
    </div>
  );
}
