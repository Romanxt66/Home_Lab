interface Opcion<T extends string> {
  valor: T;
  etiqueta: string;
}

interface Props<T extends string> {
  opciones: Opcion<T>[];
  valor: T;
  etiqueta: string;
  alCambiar(valor: T): void;
}

/** Píldoras para elegir una cosa entre pocas: pestañas y filtros. */
export function Segmentado<T extends string>({ opciones, valor, etiqueta, alCambiar }: Props<T>) {
  return (
    <div className="segmentado" role="tablist" aria-label={etiqueta}>
      {opciones.map((o) => (
        <button
          key={o.valor}
          type="button"
          role="tab"
          aria-selected={o.valor === valor}
          className={o.valor === valor ? "seleccionado" : ""}
          onClick={() => alCambiar(o.valor)}
        >
          {o.etiqueta}
        </button>
      ))}
    </div>
  );
}
