import { useLayoutEffect, useRef } from "react";
import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";

import { Glass } from "../design";
import { Icono } from "./Iconos";
import type { NombreIcono } from "./Iconos";

export interface Seccion {
  ruta: string;
  etiqueta: string;
  /** Una palabra, para la barra del móvil. */
  corta: string;
  icono: NombreIcono;
}

/**
 * Navegación superior con la píldora que viaja hasta la pestaña activa (TopNav
 * de Cristal). Es la misma pieza que la del kit, pero con los iconos propios de
 * Home Lab, que tiene secciones (bases de datos, servidor…) que el kit no dibuja.
 */
export function NavSuperior({ secciones }: { secciones: Seccion[] }) {
  const nav = useRef<HTMLElement>(null);
  const { pathname } = useLocation();

  // La píldora se mide tras cada cambio de ruta y de tamaño.
  useLayoutEffect(() => {
    const el = nav.current;
    if (!el) return;
    const colocar = () => {
      const activa = el.querySelector<HTMLElement>('a[aria-current="page"]');
      if (!activa) return el.removeAttribute("data-ind");
      el.style.setProperty("--ind-x", `${activa.offsetLeft}px`);
      el.style.setProperty("--ind-w", `${activa.offsetWidth}px`);
      el.setAttribute("data-ind", "");
    };
    colocar();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(colocar);
    ro.observe(el);
    return () => ro.disconnect();
  }, [pathname]);

  return (
    <nav ref={nav} className="cr-nav cr-nav-compacta" aria-label="Principal">
      {secciones.map((s) => (
        <NavLink key={s.ruta} to={s.ruta} end title={s.etiqueta}>
          <Icono nombre={s.icono} tamano={18} />
          <span className="cr-nav-label">{s.etiqueta}</span>
        </NavLink>
      ))}
    </nav>
  );
}

/** Barra de pestañas flotante del móvil; `final` es una pestaña más, al cabo (aquí, «más»). */
export function BarraPestanas({ secciones, final }: { secciones: Seccion[]; final?: ReactNode }) {
  return (
    <Glass as="nav" className="cr-tabbar" aria-label="Navegación principal">
      {secciones.map((s) => (
        <NavLink key={s.ruta} to={s.ruta} end className="cr-tab">
          <Icono nombre={s.icono} tamano={20} />
          {s.corta}
        </NavLink>
      ))}
      {final}
    </Glass>
  );
}
