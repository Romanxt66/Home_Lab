/** El logo de Home Lab: un cuadrado de cr-glass plateado con el pulso dentro. */
export function Marca({ tamano = 36 }: { tamano?: number }) {
  return (
    <svg width={tamano} height={tamano} viewBox="0 0 36 36" role="img" aria-label="Home Lab" style={{ flex: "none" }}>
      <defs>
        <linearGradient id="marca-fondo" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" style={{ stopColor: "var(--brand-fill)" }} />
          <stop offset="1" style={{ stopColor: "var(--brand-fill-2)" }} />
        </linearGradient>
      </defs>
      <rect width="36" height="36" rx="9" fill="url(#marca-fondo)" />
      <rect x="0.5" y="0.5" width="35" height="35" rx="8.5" fill="none" style={{ stroke: "var(--glass-border)" }} />
      <path
        d="M7 19h5l3-7 5 13 3.5-9H29"
        fill="none"
        style={{ stroke: "var(--on-brand)" }}
        strokeWidth="2.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
