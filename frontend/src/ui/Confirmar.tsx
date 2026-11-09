import { useEffect } from "react";
import { createPortal } from "react-dom";

import { Button, Glass } from "../design";
import type { ReactNode } from "react";

interface Props {
  titulo: string;
  texto: ReactNode;
  etiquetaConfirmar: string;
  /** Marca la acción como destructiva: el botón sale en rojo. */
  peligro?: boolean;
  trabajando?: boolean;
  alConfirmar(): void;
  alCancelar(): void;
}

/**
 * Confirmación propia, en lugar de window.confirm: el diálogo del navegador
 * desentona con todo lo demás y no se puede explicar bien lo que se va a
 * perder.
 */
export function Confirmar({
  titulo,
  texto,
  etiquetaConfirmar,
  peligro = false,
  trabajando = false,
  alConfirmar,
  alCancelar,
}: Props) {
  useEffect(() => {
    function alPulsar(e: KeyboardEvent) {
      if (e.key === "Escape") alCancelar();
    }
    window.addEventListener("keydown", alPulsar);
    return () => window.removeEventListener("keydown", alPulsar);
  }, [alCancelar]);

  return createPortal(
    <div className="cr cr-overlay" onMouseDown={(e) => e.target === e.currentTarget && alCancelar()}>
      <Glass strong padded className="cr-dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirmar-titulo">
        <span className="cr-page-tag">{peligro ? "no se puede deshacer" : "confirma"}</span>
        <h2 id="confirmar-titulo" className="heading" style={{ marginTop: "var(--space-3)" }}>
          {titulo}
        </h2>
        <p className="body cr-muted">{texto}</p>
        <div className="cr-dialog-actions">
          <Button variant="ghost" onClick={alCancelar}>
            Cancelar
          </Button>
          <Button variant={peligro ? "danger" : "primary"} onClick={alConfirmar} disabled={trabajando} autoFocus>
            {trabajando ? "Un momento…" : etiquetaConfirmar}
          </Button>
        </div>
      </Glass>
    </div>,
    document.body,
  );
}
