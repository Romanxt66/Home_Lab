/** «hace 3 min», «hace 2 d»… Para saber cuánto hace sin ruido de fechas exactas. */
export function hace(iso: string | null, ahora: number = Date.now()): string {
  if (!iso) return "nunca";

  const segundos = Math.max(0, Math.round((ahora - new Date(iso).getTime()) / 1000));

  if (segundos < 45) return "hace un momento";
  if (segundos < 3600) return `hace ${Math.round(segundos / 60)} min`;
  if (segundos < 86400) return `hace ${Math.round(segundos / 3600)} h`;
  if (segundos < 86400 * 30) return `hace ${Math.round(segundos / 86400)} d`;

  return new Date(iso).toLocaleDateString("es-CO", { day: "2-digit", month: "short", year: "numeric" });
}
