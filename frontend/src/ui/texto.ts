/** «Nicolás Torres» → «NT»; «ana@x.com» → «AX». */
export function iniciales(texto: string): string {
  const partes = texto.trim().split(/[\s@.]+/).filter(Boolean);
  return ((partes[0]?.[0] ?? "?") + (partes[1]?.[0] ?? "")).toUpperCase();
}
