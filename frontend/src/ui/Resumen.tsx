import type { NombreIcono } from "./Iconos";
import { Icono } from "./Iconos";
import { Cifra } from "./Cifra";

interface Props {
  etiqueta: string;
  valor: number;
  icono: NombreIcono;
  /** Clase de color: la que decide el tono del icono. */
  tono?: "acento" | "peligro" | "aviso" | "neutro";
  nota?: string;
  seleccionado?: boolean;
  alPulsar?(): void;
}

/**
 * Cifra de cabecera con su icono. Si es pulsable sirve de filtro: es más
 * directo que unos botones aparte que repitieran lo mismo.
 */
export function Resumen({
  etiqueta,
  valor,
  icono,
  tono = "neutro",
  nota,
  seleccionado = false,
  alPulsar,
}: Props) {
  const contenido = (
    <>
      <span className={`resumen-icono tono-${tono}`}>
        <Icono nombre={icono} tamano={17} />
      </span>
      <span className="resumen-datos">
        <span className="etiqueta">{etiqueta}</span>
        <span className="resumen-valor">
          <Cifra valor={valor} />
        </span>
        {nota && <span className="resumen-nota">{nota}</span>}
      </span>
    </>
  );

  if (!alPulsar) return <div className="resumen cristal">{contenido}</div>;

  return (
    <button
      type="button"
      className={`resumen resumen-boton cristal${seleccionado ? " seleccionado" : ""}`}
      onClick={alPulsar}
      aria-pressed={seleccionado}
    >
      {contenido}
    </button>
  );
}
