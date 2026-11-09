import type { ReactNode } from "react";

import { PageTitle } from "../design";

interface Props {
  /** Etiqueta pequeña sobre el titular: el nombre de la sección. */
  etiqueta: string;
  /** Titular. Lo que vaya entre asteriscos sale en serif cursiva. */
  titular: string;
  /** Frase de apoyo bajo el titular. */
  entradilla?: ReactNode;
  /** Botones a la derecha. */
  acciones?: ReactNode;
}

/**
 * Cabecera de página: etiqueta mono, titular con acento y acciones. Cada
 * pantalla abre con una, para que todas se lean igual (PageTitle de Cristal).
 */
export function Cabecera({ etiqueta, titular, entradilla, acciones }: Props) {
  return (
    <header className="cr-page-head">
      <div className="cabecera-texto">
        <PageTitle title={etiqueta} headline={titular} />
        {entradilla && <p className="body cr-muted entradilla">{entradilla}</p>}
      </div>
      {acciones && <div className="cr-actions-inline">{acciones}</div>}
    </header>
  );
}
