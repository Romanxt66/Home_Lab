import type { CSSProperties } from "react";

const TONOS: Record<string, string> = {
  running: "var(--acento)",
  stopped: "var(--texto-3)",
  degraded: "var(--peligro)",
  unknown: "var(--aviso)",
  finished: "var(--acento)",
  failed: "var(--peligro)",
  cancelled: "var(--texto-3)",
  queued: "var(--aviso)",
  in_progress: "var(--aviso)",
};

/** Ficha con un punto de color: el estado de una app, base o despliegue. */
export function FichaEstado({ estado, etiqueta }: { estado: string; etiqueta: string }) {
  return (
    <span className="ficha ficha-estado" style={{ "--tono": TONOS[estado] ?? TONOS.unknown } as CSSProperties}>
      <span className="punto" />
      {etiqueta}
    </span>
  );
}
