import type { ReactNode } from "react";

interface Props {
  /** Etiqueta pequeña sobre el titular: el nombre de la sección. */
  etiqueta: string;
  /** Titular. Lo que vaya entre asteriscos sale en serif cursiva y color de acento. */
  titular: string;
  /** Frase de apoyo bajo el titular. */
  entradilla?: ReactNode;
  /** Botones a la derecha. */
  acciones?: ReactNode;
}

/**
 * Cabecera de página: etiqueta mono, titular con acento y acciones.
 * Cada pantalla abre con una, para que todas se lean igual.
 */
export function Cabecera({ etiqueta, titular, entradilla, acciones }: Props) {
  return (
    <header className="cabecera subir">
      <div className="cabecera-texto">
        <span className="etiqueta">{etiqueta}</span>
        <h1>{conAcento(titular)}</h1>
        {entradilla && <p className="entradilla">{entradilla}</p>}
      </div>
      {acciones && <div className="cabecera-acciones">{acciones}</div>}
    </header>
  );
}

/** «Tu panel, *de un vistazo*.» → los tramos impares van en cursiva. */
function conAcento(texto: string): ReactNode[] {
  return texto
    .split("*")
    .map((trozo, i) => (i % 2 === 1 ? <em key={i}>{trozo}</em> : trozo));
}
