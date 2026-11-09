import type { SelectHTMLAttributes } from "react";

import { Select } from "../design";

/**
 * Desplegable de Cristal: un <select> nativo transparente sobre una cara de
 * vidrio (accesible, y nativo en el móvil) que en escritorio abre un menú de
 * cr-glass propio. Se usa igual que un <select>.
 *
 * La etiqueta visible la pone el `.campo` que lo envuelve; la del kit se
 * oculta por CSS para no repetirla.
 */
export function Selector({ children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className="selector">
      <Select label={props["aria-label"] ?? ""} {...props}>
        {children}
      </Select>
    </div>
  );
}
