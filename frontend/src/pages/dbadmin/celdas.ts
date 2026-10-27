/** Cómo se enseña un valor que llega de la base: NULL aparte y los objetos como JSON. */
export function textoDeCelda(v: unknown): string {
  if (v === null || v === undefined) return "NULL";
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

export const esNulo = (v: unknown) => v === null || v === undefined;

/** Los tipos que pueden ser largos y conviene editar en un área de texto. */
export function esTextoLargo(tipo: string): boolean {
  return /json|text|xml/i.test(tipo);
}

export function nombreCompleto(schema: string, tabla: string): string {
  return `${schema}.${tabla}`;
}
