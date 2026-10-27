import { useState } from "react";

import { iniciales } from "./texto";

interface Props {
  nombre: string;
  /** Foto, si la hay (la que da Google). */
  imagen?: string | null;
  tamano?: number;
}

/**
 * Foto o, si no hay o no carga, las iniciales. La foto vive en un servidor de
 * Google que puede estar bloqueado o caído: sin el respaldo, quedaría un
 * hueco roto en la lista.
 */
export function Avatar({ nombre, imagen, tamano = 32 }: Props) {
  const [fallo, setFallo] = useState(false);

  return (
    <span
      className="avatar"
      style={{ width: tamano, height: tamano, fontSize: tamano * 0.38 }}
      aria-hidden="true"
    >
      {imagen && !fallo ? (
        <img
          src={imagen}
          alt=""
          width={tamano}
          height={tamano}
          // No se manda de qué página se pide la foto a un servidor ajeno.
          referrerPolicy="no-referrer"
          onError={() => setFallo(true)}
        />
      ) : (
        iniciales(nombre)
      )}
    </span>
  );
}
