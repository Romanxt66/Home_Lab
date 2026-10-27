import { useEffect, useState } from "react";

/** Si el sistema pide menos movimiento, las cifras aparecen ya puestas. */
function reducirMovimiento(): boolean {
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

interface Props {
  valor: number;
  /** Decimales a mostrar. */
  decimales?: number;
  sufijo?: string;
}

/**
 * Número que sube de 0 hasta su valor. Dura menos de un segundo y frena al
 * final (ease-out): se nota que está vivo sin hacer esperar a nadie.
 */
export function Cifra({ valor, decimales = 0, sufijo = "" }: Props) {
  const reducido = reducirMovimiento();
  const [animado, setAnimado] = useState(0);

  // Con «menos movimiento» no hay animación: el valor se pinta directo, sin
  // pasar por el estado.
  const mostrado = reducido ? valor : animado;

  useEffect(() => {
    if (reducido) return;

    const duracion = 900;
    const inicio = performance.now();
    let cuadro = 0;

    const paso = (ahora: number) => {
      // El primer fotograma puede traer una marca anterior al inicio: sin el
      // Math.max, el progreso sale negativo y un valor 0 se pinta como «-0».
      const t = Math.min(Math.max((ahora - inicio) / duracion, 0), 1);
      const suavizado = 1 - Math.pow(1 - t, 3);
      setAnimado(valor * suavizado);
      if (t < 1) cuadro = requestAnimationFrame(paso);
    };
    cuadro = requestAnimationFrame(paso);

    return () => cancelAnimationFrame(cuadro);
  }, [valor, reducido]);

  return (
    <span className="cifra">
      {(mostrado || 0).toLocaleString("es-CO", {
        minimumFractionDigits: decimales,
        maximumFractionDigits: decimales,
      })}
      {sufijo}
    </span>
  );
}
