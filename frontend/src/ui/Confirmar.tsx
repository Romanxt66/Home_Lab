import { useEffect } from "react";
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

  return (
    <div className="fondo-dialogo" onMouseDown={alCancelar}>
      <div
        className="dialogo dialogo-estrecho"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirmar-titulo"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="dialogo-cuerpo">
          <span className="etiqueta">{peligro ? "no se puede deshacer" : "confirma"}</span>
          <h2 id="confirmar-titulo" className="dialogo-titular">
            {titulo}
          </h2>
          <p className="dialogo-texto">{texto}</p>
        </div>
        <div className="dialogo-pie">
          <button type="button" className="btn" onClick={alCancelar}>
            Cancelar
          </button>
          <button
            type="button"
            className={`btn ${peligro ? "btn-peligro-solido" : "btn-principal"}`}
            onClick={alConfirmar}
            disabled={trabajando}
            autoFocus
          >
            {trabajando ? "Un momento…" : etiquetaConfirmar}
          </button>
        </div>
      </div>
    </div>
  );
}
