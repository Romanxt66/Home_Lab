/** 1.536 → «1,5 KB». */
export function tamano(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toLocaleString("es-CO", { maximumFractionDigits: 1 })} KB`;
  return `${(bytes / 1024 / 1024).toLocaleString("es-CO", { maximumFractionDigits: 1 })} MB`;
}
