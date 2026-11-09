import { Link } from "react-router-dom";

import { useSesion } from "../api/sesion";
import { useSecciones } from "../secciones";
import { Cabecera } from "../ui/Cabecera";
import { Icono } from "../ui/Iconos";

/** Portada: un acceso a cada módulo, con lo que se hace en él. */
export function Inicio() {
  const { usuario } = useSesion();
  const secciones = useSecciones().filter((s) => s.ruta !== "/");
  const nombre = (usuario?.name ?? usuario?.email ?? "").split(" ")[0];

  return (
    <div className="pagina">
      <Cabecera
        etiqueta="inicio"
        titular={nombre ? `Hola, *${nombre}*.` : "Tu laboratorio, *a mano*."}
        entradilla="Elige una herramienta para empezar."
      />

      <div className="cr-grid-cols">
        {secciones.map((s, i) => (
          <Link
            key={s.ruta}
            to={s.ruta}
            className="cr-glass cr-glass-md cr-pad acceso-modulo subir"
            style={{ "--i": Math.min(i, 8) } as React.CSSProperties}
          >
            <span className="acceso-modulo-icono">
              <Icono nombre={s.icono} tamano={20} />
            </span>
            <span className="acceso-modulo-texto">
              <span className="heading">{s.etiqueta}</span>
              <span className="label cr-muted">{s.detalle}</span>
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}
