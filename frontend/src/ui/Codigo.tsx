import { useState } from "react";

import { Icono } from "./Iconos";

interface Props {
  titulo: string;
  texto: string;
  /** Un botón más junto a «Copiar», p. ej. «Subir al repositorio». */
  extra?: React.ReactNode;
}

/** Bloque de código con su título y un botón para copiarlo. */
export function Codigo({ titulo, texto, extra }: Props) {
  const [copiado, setCopiado] = useState(false);

  async function copiar() {
    try {
      await navigator.clipboard.writeText(texto);
      setCopiado(true);
      setTimeout(() => setCopiado(false), 1800);
    } catch {
      // Sin permiso para el portapapeles: el texto sigue ahí para seleccionarlo.
    }
  }

  return (
    <div className="codigo cristal">
      <div className="codigo-cab">
        <span className="etiqueta">{titulo}</span>
        <div className="codigo-acciones">
          {extra}
          <button type="button" className="btn btn-pequeno" onClick={() => void copiar()}>
            <Icono nombre={copiado ? "check" : "copy"} tamano={13} />
            {copiado ? "Copiado" : "Copiar"}
          </button>
        </div>
      </div>
      <pre tabIndex={0}>{texto}</pre>
    </div>
  );
}
