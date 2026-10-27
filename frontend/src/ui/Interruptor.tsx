interface Props {
  activo: boolean;
  /** Qué enciende o apaga, para quien usa lector de pantalla. */
  etiqueta: string;
  deshabilitado?: boolean;
  alCambiar(activo: boolean): void;
}

/** Interruptor encendido/apagado. Es un botón con rol «switch», no una casilla disfrazada. */
export function Interruptor({ activo, etiqueta, deshabilitado = false, alCambiar }: Props) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={activo}
      aria-label={etiqueta}
      title={activo ? "Activa: pulsa para pausar" : "En pausa: pulsa para activar"}
      className={`interruptor${activo ? " activo" : ""}`}
      disabled={deshabilitado}
      onClick={() => alCambiar(!activo)}
    >
      <span className="interruptor-bola" />
    </button>
  );
}
