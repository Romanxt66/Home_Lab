import { Segmented } from "../design";

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

/** Píldoras para elegir una cosa entre pocas: pestañas y filtros (Segmented de Cristal). */
export function Segmentado<T extends string>({ opciones, valor, etiqueta, alCambiar }: Props<T>) {
  return (
    <Segmented
      label={etiqueta}
      value={valor}
      onChange={alCambiar}
      options={opciones.map((o) => ({ value: o.valor, label: o.etiqueta }))}
    />
  );
}
