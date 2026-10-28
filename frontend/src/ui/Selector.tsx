import { Children, isValidElement, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent, ReactElement, ReactNode } from "react";
import { createPortal } from "react-dom";

import { Icono } from "./Iconos";

interface Props {
  id?: string;
  value: string;
  onChange(e: { target: { value: string } }): void;
  /** Los mismos <option> que llevaría un <select>. */
  children: ReactNode;
  disabled?: boolean;
  required?: boolean;
  "aria-label"?: string;
}

interface Opcion {
  valor: string;
  etiqueta: string;
}

/** Saca las opciones de los <option> hijos, ignorando lo que React deja en falso. */
function leerOpciones(hijos: ReactNode): Opcion[] {
  const salida: Opcion[] = [];
  Children.forEach(hijos, (h) => {
    if (!isValidElement(h)) return;
    const el = h as ReactElement<{ value?: string; children?: ReactNode }>;
    if (el.type === "option") {
      const etiqueta = Children.toArray(el.props.children).join("");
      salida.push({ valor: el.props.value ?? etiqueta, etiqueta });
    } else {
      // Un fragmento o un grupo con más <option> dentro.
      salida.push(...leerOpciones(el.props.children));
    }
  });
  return salida;
}

/**
 * Sustituto de <select> con la lista dibujada por la aplicación. La lista
 * nativa la pinta el sistema (azul y blanca en Linux y Windows) y desentona con
 * todo lo demás; esta usa los mismos colores que el resto de las superficies.
 *
 * Mismo uso que un <select>: <Selector value onChange>…<option/>…</Selector>.
 */
export function Selector({ id, value, onChange, children, disabled = false, required = false, "aria-label": etiqueta }: Props) {
  const opciones = leerOpciones(children);
  const idLista = useId();
  const boton = useRef<HTMLButtonElement>(null);
  const lista = useRef<HTMLUListElement>(null);
  const [abierto, setAbierto] = useState(false);
  const [activa, setActiva] = useState(0);
  const [caja, setCaja] = useState<{ left: number; top: number; width: number; maxHeight: number; arriba: boolean } | null>(null);

  const indiceActual = Math.max(0, opciones.findIndex((o) => o.valor === value));
  const seleccionada = opciones.find((o) => o.valor === value);

  // La lista va en un portal y con posición fija: dentro de un diálogo con
  // desplazamiento, una lista absoluta quedaría recortada.
  useLayoutEffect(() => {
    if (!abierto || !boton.current) return;
    const r = boton.current.getBoundingClientRect();
    const debajo = window.innerHeight - r.bottom - 12;
    const encima = r.top - 12;
    const arriba = debajo < 180 && encima > debajo;
    const espacio = arriba ? encima : debajo;
    setCaja({
      left: r.left,
      width: r.width,
      top: arriba ? r.top - 6 : r.bottom + 6,
      maxHeight: Math.max(120, Math.min(280, espacio)),
      arriba,
    });
  }, [abierto]);

  useEffect(() => {
    if (!abierto) return;
    const fuera = (e: PointerEvent) => {
      const t = e.target as Node;
      if (!boton.current?.contains(t) && !lista.current?.contains(t)) setAbierto(false);
    };
    // Al desplazarse la página la lista se quedaría suelta: se cierra.
    const cerrar = (e: Event) => {
      if (lista.current?.contains(e.target as Node)) return;
      setAbierto(false);
    };
    document.addEventListener("pointerdown", fuera);
    window.addEventListener("scroll", cerrar, true);
    window.addEventListener("resize", cerrar);
    return () => {
      document.removeEventListener("pointerdown", fuera);
      window.removeEventListener("scroll", cerrar, true);
      window.removeEventListener("resize", cerrar);
    };
  }, [abierto]);

  // Mantiene a la vista la opción activa al moverse con el teclado.
  useEffect(() => {
    if (abierto) lista.current?.children[activa]?.scrollIntoView({ block: "nearest" });
  }, [abierto, activa, caja]);

  function abrir() {
    if (disabled) return;
    setActiva(indiceActual);
    setAbierto(true);
  }

  function elegir(i: number) {
    const o = opciones[i];
    if (o) onChange({ target: { value: o.valor } });
    setAbierto(false);
    boton.current?.focus();
  }

  function alPulsar(e: KeyboardEvent) {
    if (!abierto) {
      if (["ArrowDown", "ArrowUp", "Enter", " "].includes(e.key)) {
        e.preventDefault();
        abrir();
      }
      return;
    }
    switch (e.key) {
      case "ArrowDown": e.preventDefault(); setActiva((i) => Math.min(opciones.length - 1, i + 1)); break;
      case "ArrowUp": e.preventDefault(); setActiva((i) => Math.max(0, i - 1)); break;
      case "Home": e.preventDefault(); setActiva(0); break;
      case "End": e.preventDefault(); setActiva(opciones.length - 1); break;
      case "Enter":
      case " ": e.preventDefault(); elegir(activa); break;
      case "Escape":
        // Solo cierra la lista: sin esto también se cerraría el diálogo que la contiene.
        e.preventDefault();
        e.stopPropagation();
        setAbierto(false);
        break;
      case "Tab": setAbierto(false); break;
      default: {
        // Escribir una letra salta a la siguiente opción que empiece por ella.
        if (e.key.length === 1) {
          const l = e.key.toLowerCase();
          const sig = opciones.findIndex((o, i) => i > activa && o.etiqueta.toLowerCase().startsWith(l));
          const desde0 = opciones.findIndex((o) => o.etiqueta.toLowerCase().startsWith(l));
          const i = sig >= 0 ? sig : desde0;
          if (i >= 0) setActiva(i);
        }
      }
    }
  }

  const estiloLista: CSSProperties | undefined = caja
    ? {
        left: caja.left,
        width: caja.width,
        maxHeight: caja.maxHeight,
        ...(caja.arriba ? { bottom: window.innerHeight - caja.top } : { top: caja.top }),
      }
    : undefined;

  return (
    <div className="selector">
      <button
        ref={boton}
        id={id}
        type="button"
        className="selector-boton"
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={abierto}
        aria-controls={idLista}
        aria-label={etiqueta}
        aria-activedescendant={abierto ? `${idLista}-${activa}` : undefined}
        disabled={disabled}
        onClick={() => (abierto ? setAbierto(false) : abrir())}
        onKeyDown={alPulsar}
      >
        <span className={seleccionada ? "" : "selector-vacio"}>{seleccionada?.etiqueta ?? "—"}</span>
        <Icono nombre="chevronR" tamano={15} style={{ transform: `rotate(${abierto ? -90 : 90}deg)` }} />
      </button>

      {/* Para que `required` siga bloqueando el envío del formulario. */}
      {required && <input className="selector-requerido" tabIndex={-1} aria-hidden value={value} onChange={() => undefined} required />}

      {abierto &&
        estiloLista &&
        createPortal(
          <ul ref={lista} id={idLista} role="listbox" className="selector-lista cristal" style={estiloLista}>
            {opciones.map((o, i) => (
              <li
                key={o.valor + i}
                id={`${idLista}-${i}`}
                role="option"
                aria-selected={o.valor === value}
                className={`${i === activa ? "activa " : ""}${o.valor === value ? "elegida" : ""}`}
                onPointerEnter={() => setActiva(i)}
                onClick={() => elegir(i)}
              >
                <span>{o.etiqueta}</span>
                {o.valor === value && <Icono nombre="check" tamano={14} />}
              </li>
            ))}
          </ul>,
          document.body,
        )}
    </div>
  );
}
