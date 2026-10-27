import { useEffect, useState } from "react";

import { api } from "../api/client";

interface Estado<T> {
  clave: string;
  datos: T | null;
  error: string | null;
}

/**
 * Pide una ruta de la API y la vuelve a pedir cuando cambia o cuando se llama
 * a `recargar`. Pasar `null` en la ruta es «todavía no toca»: no pide nada.
 *
 * Mientras llega la respuesta nueva se siguen enseñando los datos de la
 * anterior, para que la pantalla no parpadee a «cargando» cada vez que se
 * cambia un filtro. Y si se cambia de ruta rápido, la respuesta lenta de la
 * anterior se descarta en vez de pisar la buena.
 */
export function useDatos<T>(ruta: string | null) {
  const [version, setVersion] = useState(0);
  const [estado, setEstado] = useState<Estado<T> | null>(null);

  const clave = `${ruta}|${version}`;

  useEffect(() => {
    if (ruta === null) return;

    let vigente = true;
    api
      .get<T>(ruta)
      .then((datos) => {
        if (vigente) setEstado({ clave, datos, error: null });
      })
      .catch((e) => {
        if (!vigente) return;
        setEstado((anterior) => ({
          clave,
          datos: anterior?.datos ?? null,
          error: e instanceof Error ? e.message : "No se pudo cargar.",
        }));
      });

    return () => {
      vigente = false;
    };
  }, [ruta, clave]);

  return {
    datos: estado?.datos ?? null,
    error: estado?.error ?? null,
    cargando: ruta !== null && estado?.clave !== clave,
    recargar: () => setVersion((v) => v + 1),
  };
}
