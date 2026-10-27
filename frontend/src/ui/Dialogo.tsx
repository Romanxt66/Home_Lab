import { useEffect } from "react";
import type { FormEvent, ReactNode } from "react";

import { Icono } from "./Iconos";

interface Props {
  /** Etiqueta mono sobre el titular: «nuevo», «editar»… */
  etiqueta: string;
  /** Titular. Lo que vaya entre asteriscos sale en serif cursiva. */
  titular: string;
  /** Mensaje de error del servidor, ya escrito para que lo lea una persona. */
  error?: string | null;
  enviando?: boolean;
  etiquetaEnviar?: string;
  /** Un botón a la izquierda del pie, p. ej. «Eliminar». */
  accionIzquierda?: ReactNode;
  alEnviar(): void;
  alCerrar(): void;
  children: ReactNode;
}

/**
 * Diálogo de formulario: cabecera con titular, cuerpo y pie con Cancelar y
 * Guardar. Cierra con Escape o pulsando fuera, que es lo que todo el mundo
 * intenta primero.
 */
export function Dialogo({
  etiqueta,
  titular,
  error,
  enviando = false,
  etiquetaEnviar = "Guardar",
  accionIzquierda,
  alEnviar,
  alCerrar,
  children,
}: Props) {
  useEffect(() => {
    function alPulsar(e: KeyboardEvent) {
      if (e.key === "Escape") alCerrar();
    }
    window.addEventListener("keydown", alPulsar);
    return () => window.removeEventListener("keydown", alPulsar);
  }, [alCerrar]);

  function enviar(e: FormEvent) {
    e.preventDefault();
    alEnviar();
  }

  return (
    <div className="fondo-dialogo" onMouseDown={alCerrar}>
      {/* Un clic dentro no debe cerrarlo. */}
      <div
        className="dialogo"
        role="dialog"
        aria-modal="true"
        aria-labelledby="dialogo-titulo"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <form onSubmit={enviar}>
          <div className="dialogo-cab">
            <div className="dialogo-cab-texto">
              <span className="etiqueta">{etiqueta}</span>
              <h2 id="dialogo-titulo" className="dialogo-titular">
                {titular.split("*").map((t, i) => (i % 2 === 1 ? <em key={i}>{t}</em> : t))}
              </h2>
            </div>
            <button type="button" className="accion" onClick={alCerrar} aria-label="Cerrar" title="Cerrar">
              <Icono nombre="close" tamano={18} />
            </button>
          </div>

          <div className="dialogo-cuerpo">
            {error && (
              <div className="alerta alerta-error" role="alert">
                <Icono nombre="alert" tamano={16} />
                <span>{error}</span>
              </div>
            )}
            {children}
          </div>

          <div className="dialogo-pie dialogo-pie-partido">
            <div>{accionIzquierda}</div>
            <div className="dialogo-pie-botones">
              <button type="button" className="btn" onClick={alCerrar}>
                Cancelar
              </button>
              <button type="submit" className="btn btn-principal" disabled={enviando}>
                {enviando ? "Guardando…" : etiquetaEnviar}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
